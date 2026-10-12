package httpsig

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

var (
	now     = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	keyID   = "https://mastodon.example/users/alice#main-key"
	body    = []byte(`{"type":"Follow","actor":"https://mastodon.example/users/alice"}`)
	testKey = mustKey()
)

func mustKey() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
}

// As it is sent, and then as the server that receives it sees it.
func sent(t *testing.T, method, target string, payload []byte) *http.Request {
	t.Helper()

	out, err := http.NewRequest(method, target, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if err := Sign(out, payload, keyID, testKey, now); err != nil {
		t.Fatal(err)
	}

	in, _ := http.NewRequest(method, out.URL.RequestURI(), bytes.NewReader(payload))
	in.Host = out.Host
	in.Header = out.Header.Clone()
	return in
}

func verify(req *http.Request, payload []byte, at time.Time, key *rsa.PublicKey) error {
	sig, err := Parse(req)
	if err != nil {
		return err
	}
	return Verify(req, payload, sig, key, at)
}

func TestWhatIsSignedChecks(t *testing.T) {
	post := sent(t, http.MethodPost, "https://api.example.org/inbox", body)
	if err := verify(post, body, now, &testKey.PublicKey); err != nil {
		t.Fatalf("a post: %v", err)
	}
	sig, _ := Parse(post)
	if sig.KeyID != keyID || strings.Join(sig.Headers, " ") != "(request-target) host date digest" {
		t.Errorf("read as %+v", sig)
	}

	get := sent(t, http.MethodGet, "https://api.example.org/ap/users/1?page=2", nil)
	if err := verify(get, nil, now, &testKey.PublicKey); err != nil {
		t.Fatalf("a get: %v", err)
	}
	if get.Header.Get("Digest") != "" {
		t.Error("a request with no body was given a digest")
	}

	// A little late, and a clock a little ahead, are both fine.
	for _, at := range []time.Time{now.Add(MaxSkew - time.Second), now.Add(-MaxSkew + time.Second)} {
		if err := verify(post, body, at, &testKey.PublicKey); err != nil {
			t.Errorf("%v from when it was signed: %v", at.Sub(now), err)
		}
	}
}

func TestWhatWasNotSignedDoesNot(t *testing.T) {
	other := mustKey()

	cases := map[string]struct {
		change func(req *http.Request) []byte
		at     time.Time
		key    *rsa.PublicKey
		want   error
	}{
		"another body": {func(r *http.Request) []byte {
			return []byte(`{"type":"Delete","actor":"https://mastodon.example/users/alice"}`)
		}, now, &testKey.PublicKey, ErrDigest},
		"another body with its own digest": {func(r *http.Request) []byte {
			swapped := []byte(`{"type":"Delete"}`)
			r.Header.Set("Digest", Digest(swapped))
			return swapped
		}, now, &testKey.PublicKey, ErrBadSignature},
		"sent to another address": {func(r *http.Request) []byte {
			r.URL.Path = "/ap/users/somebody-else/inbox"
			return body
		}, now, &testKey.PublicKey, ErrBadSignature},
		"sent to another server": {func(r *http.Request) []byte {
			r.Host = "other.example"
			return body
		}, now, &testKey.PublicKey, ErrBadSignature},
		"sent again the next day": {func(r *http.Request) []byte { return body }, now.Add(24 * time.Hour), &testKey.PublicKey, ErrDate},
		"dated in the future":     {func(r *http.Request) []byte { return body }, now.Add(-time.Hour), &testKey.PublicKey, ErrDate},
		"given a new date": {func(r *http.Request) []byte {
			r.Header.Set("Date", now.Add(time.Minute).Format(http.TimeFormat))
			return body
		}, now, &testKey.PublicKey, ErrBadSignature},
		"no date": {func(r *http.Request) []byte {
			r.Header.Del("Date")
			return body
		}, now, &testKey.PublicKey, ErrDate},
		"somebody else's key": {func(r *http.Request) []byte { return body }, now, &other.PublicKey, ErrBadSignature},
		"not covering the digest": {func(r *http.Request) []byte {
			r.Header.Set("Signature", strings.Replace(r.Header.Get("Signature"), "host date digest", "host date", 1))
			return body
		}, now, &testKey.PublicKey, ErrNotCovered},
		"not covering the host": {func(r *http.Request) []byte {
			r.Header.Set("Signature", strings.Replace(r.Header.Get("Signature"), "(request-target) host date", "(request-target) date", 1))
			return body
		}, now, &testKey.PublicKey, ErrNotCovered},
		"covering only the date": {func(r *http.Request) []byte {
			r.Header.Set("Signature", strings.Replace(r.Header.Get("Signature"), "(request-target) host date digest", "date", 1))
			return body
		}, now, &testKey.PublicKey, ErrNotCovered},
		"a kind of signature not accepted": {func(r *http.Request) []byte {
			r.Header.Set("Signature", strings.Replace(r.Header.Get("Signature"), `algorithm="rsa-sha256"`, `algorithm="hmac-sha256"`, 1))
			return body
		}, now, &testKey.PublicKey, ErrAlgorithm},
		"covering a header that was not sent": {func(r *http.Request) []byte {
			r.Header.Set("Signature", strings.Replace(r.Header.Get("Signature"), "host date digest", "host date digest x-missing", 1))
			return body
		}, now, &testKey.PublicKey, ErrNotCovered},
	}

	for name, c := range cases {
		req := sent(t, http.MethodPost, "https://api.example.org/inbox", body)
		payload := c.change(req)
		if err := verify(req, payload, c.at, c.key); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
}

func TestASignatureThatCannotBeReadIsRefused(t *testing.T) {
	for header, want := range map[string]error{
		"":                         ErrNoSignature,
		`keyId="a",headers="date"`: ErrMalformed,
		`keyId="a",headers="date",signature="not base64!"`:    ErrMalformed,
		`keyId=a,headers="date",signature="YQ=="`:             ErrMalformed,
		`headers="date",signature="YQ=="`:                     ErrMalformed,
		`keyId="a",keyId="b",headers="date",signature="YQ=="`: ErrMalformed,
		`nonsense`: ErrMalformed,
	} {
		req, _ := http.NewRequest(http.MethodPost, "/inbox", nil)
		if header != "" {
			req.Header.Set("Signature", header)
		}
		if _, err := Parse(req); !errors.Is(err, want) {
			t.Errorf("%q: got %v, want %v", header, err, want)
		}
	}

	// A comma inside a quoted value is part of the value.
	req, _ := http.NewRequest(http.MethodPost, "/inbox", nil)
	req.Header.Set("Signature", `keyId="https://a.example/u?x=1,2#main-key",algorithm="hs2019",headers="(request-target) host date digest",signature="YQ=="`)
	sig, err := Parse(req)
	if err != nil || sig.KeyID != "https://a.example/u?x=1,2#main-key" || sig.Algorithm != "hs2019" {
		t.Errorf("read as %+v (%v)", sig, err)
	}

	// Sent as an Authorization, as some do.
	req, _ = http.NewRequest(http.MethodGet, "/ap/users/1", nil)
	req.Header.Set("Authorization", `Signature keyId="k",headers="(request-target) host date",signature="YQ=="`)
	if sig, err := Parse(req); err != nil || sig.KeyID != "k" {
		t.Errorf("as an authorization: %+v (%v)", sig, err)
	}
}
