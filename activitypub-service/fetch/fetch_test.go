package fetch

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"activitypub/httpsig"
)

var key, _ = rsa.GenerateKey(rand.Reader, 2048)

func signer() (string, *rsa.PrivateKey, error) {
	return "https://api.example.org/ap/actor#main-key", key, nil
}

func TestAFetchIsSignedAtEveryHop(t *testing.T) {
	var asked []string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Each request, redirected or not, is signed for where it went.
		body, _ := io.ReadAll(r.Body)
		sig, err := httpsig.Parse(r)
		if err != nil {
			http.Error(w, "unsigned", http.StatusUnauthorized)
			return
		}
		if err := httpsig.Verify(r, body, sig, &key.PublicKey, time.Now()); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		asked = append(asked, r.URL.Path+" "+r.Header.Get("Accept")[:25])

		switch r.URL.Path {
		case "/old":
			http.Redirect(w, r, "/users/alice", http.StatusFound)
		case "/users/alice":
			w.Header().Set("Content-Type", "application/activity+json")
			_, _ = w.Write([]byte(`{"id":"x"}`))
		case "/gone":
			w.WriteHeader(http.StatusGone)
		case "/page":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html>`))
		case "/huge":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"a":"` + strings.Repeat("x", MaxBytes) + `"}`))
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/down":
			http.Redirect(w, r, "http://"+r.Host+"/users/alice", http.StatusFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := newFrom(server.Client(), signer)

	var got struct{ ID string }
	from, err := client.Object(context.Background(), server.URL+"/old", &got)
	if err != nil || got.ID != "x" || from != server.URL+"/users/alice" {
		t.Fatalf("got %+v from %q (%v)", got, from, err)
	}
	if len(asked) != 2 || !strings.HasPrefix(asked[0], "/old application/activity+json") {
		t.Errorf("asked %v", asked)
	}

	for path, want := range map[string]error{
		"/gone":  ErrGone,
		"/page":  ErrUnreadable,
		"/huge":  ErrUnreadable,
		"/loop":  ErrUnreadable,
		"/error": ErrUnreadable,
		"/down":  ErrNotAllowed,
	} {
		if _, err := client.Object(context.Background(), server.URL+path, &got); !errors.Is(err, want) {
			t.Errorf("%s: got %v, want %v", path, err, want)
		}
	}
}

func TestOnlyHTTPSAndOnlyPublicAddresses(t *testing.T) {
	client := New("test", signer)

	var got map[string]any
	for _, address := range []string{
		"http://mastodon.example/users/alice",
		"ftp://mastodon.example/users/alice",
		"https://user:pass@mastodon.example/users/alice",
		"/users/alice",
		"",
	} {
		if _, err := client.Object(context.Background(), address, &got); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%q: %v", address, err)
		}
	}

	// A name is no protection: it is the address connected to that is
	// judged. These are this machine and the network it is on.
	for _, address := range []string{"https://127.0.0.1/users/alice", "https://10.0.0.5/x", "https://169.254.169.254/latest/meta-data", "https://[::1]/x", "https://localhost/x"} {
		if _, err := client.Object(context.Background(), address, &got); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%q: %v", address, err)
		}
	}

	// The lab's rule lets private addresses through, on the secure port
	// only.
	if !LabAddresses(net.ParseIP("172.20.1.50"), "443") || LabAddresses(net.ParseIP("172.20.1.50"), "3306") || LabAddresses(nil, "443") {
		t.Error("the lab's rule is wrong")
	}
}
