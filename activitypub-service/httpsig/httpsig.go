// Package httpsig signs requests to other servers and checks the signature
// on theirs, as the fediverse does it.
//
// A server that receives "ana followed you" has only the request to go on.
// What makes it believable is a signature over the request made with ana's
// key, which the receiver checks against the public half it reads from
// ana's description. Without that check, anybody could send anything in
// anybody's name.
//
// The scheme is the one Mastodon settled on and everybody since has had to
// speak: draft-cavage-http-signatures, with RSA and SHA-256, over the
// request's target, its host, its date and, when it has a body, a digest of
// the body. It is small and fixed, so it is written out here and not taken
// from a library: every line of it is something an attacker will try.
package httpsig

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// MaxSkew is how far a request's date may be from now, either way. Long
// enough for clocks that disagree and a slow network; short enough that a
// request somebody recorded is of no use to them the next day.
const MaxSkew = 15 * time.Minute

const algorithm = "rsa-sha256"

var (
	ErrNoSignature  = errors.New("the request is not signed")
	ErrMalformed    = errors.New("the signature cannot be read")
	ErrNotCovered   = errors.New("the signature does not cover what it must")
	ErrDate         = errors.New("the request's date is missing or too far from now")
	ErrDigest       = errors.New("the body is not the one that was signed")
	ErrAlgorithm    = errors.New("the signature is of a kind that is not accepted")
	ErrBadSignature = errors.New("the signature is not this key's")
)

// Signature is what a Signature header says.
type Signature struct {
	// KeyID names the key: the signer's address, and which key of theirs.
	KeyID     string
	Algorithm string
	// Headers is what was signed, in order.
	Headers []string
	Value   []byte
}

// Digest is the header that says what the body is.
func Digest(body []byte) string {
	sum := sha256.Sum256(body)
	return "SHA-256=" + base64.StdEncoding.EncodeToString(sum[:])
}

// Sign signs a request that is about to be sent, with the key named. It
// sets Date, Host, and Digest when there is a body, and then Signature.
func Sign(req *http.Request, body []byte, keyID string, key *rsa.PrivateKey, now time.Time) error {
	if req.URL == nil || req.URL.Host == "" {
		return errors.New("a request with nowhere to go cannot be signed")
	}

	req.Header.Set("Date", now.UTC().Format(http.TimeFormat))
	req.Host = req.URL.Host

	headers := []string{"(request-target)", "host", "date"}
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		req.Header.Set("Digest", Digest(body))
		headers = append(headers, "digest")
	}

	signed, err := signingString(req, headers)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(signed))
	value, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return err
	}

	req.Header.Set("Signature", fmt.Sprintf(
		`keyId="%s",algorithm="%s",headers="%s",signature="%s"`,
		keyID, algorithm, strings.Join(headers, " "), base64.StdEncoding.EncodeToString(value),
	))
	return nil
}

// Parse reads a request's Signature header. It checks nothing but that it
// can be read: its use is to learn whose key to fetch.
func Parse(req *http.Request) (*Signature, error) {
	header := req.Header.Get("Signature")
	if header == "" {
		// Some send it as an Authorization of this kind.
		if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Signature ") {
			header = strings.TrimPrefix(auth, "Signature ")
		}
	}
	if header == "" {
		return nil, ErrNoSignature
	}

	fields := map[string]string{}
	for _, part := range splitParams(header) {
		name, value, found := strings.Cut(part, "=")
		if !found {
			return nil, ErrMalformed
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
			return nil, ErrMalformed
		}
		if _, twice := fields[name]; twice {
			return nil, ErrMalformed
		}
		fields[name] = value[1 : len(value)-1]
	}

	sig := &Signature{KeyID: fields["keyId"], Algorithm: strings.ToLower(fields["algorithm"])}
	if sig.KeyID == "" || fields["signature"] == "" || fields["headers"] == "" {
		return nil, ErrMalformed
	}

	value, err := base64.StdEncoding.DecodeString(fields["signature"])
	if err != nil {
		return nil, ErrMalformed
	}
	sig.Value = value
	sig.Headers = strings.Fields(strings.ToLower(fields["headers"]))

	return sig, nil
}

// Verify checks that a request was signed by the holder of a key, and that
// what was signed is this request: this address, this host, this body, now.
//
// EVERY ONE OF THESE CHECKS IS ONE AN ATTACK GOES THROUGH WITHOUT.
//   - Not covering the target lets a signature made for one address be sent
//     to another.
//   - Not covering the host lets what was signed for one server be sent to
//     a different one.
//   - Not covering the date, or not minding how old it is, lets a recorded
//     request be sent again for ever.
//   - Not covering the digest, or not comparing it with the body, lets the
//     body be swapped for another under a signature that still checks.
func Verify(req *http.Request, body []byte, sig *Signature, key *rsa.PublicKey, now time.Time) error {
	// hs2019 says "whatever the key is", which for the keys there are is
	// this.
	if sig.Algorithm != "" && sig.Algorithm != algorithm && sig.Algorithm != "hs2019" {
		return ErrAlgorithm
	}

	covered := map[string]bool{}
	for _, header := range sig.Headers {
		covered[header] = true
	}
	hasBody := req.Method != http.MethodGet && req.Method != http.MethodHead
	if !covered["(request-target)"] || !covered["host"] || !covered["date"] || (hasBody && !covered["digest"]) {
		return ErrNotCovered
	}

	date, err := http.ParseTime(req.Header.Get("Date"))
	if err != nil {
		return ErrDate
	}
	if skew := now.Sub(date); skew > MaxSkew || skew < -MaxSkew {
		return ErrDate
	}

	if hasBody {
		if subtle.ConstantTimeCompare([]byte(req.Header.Get("Digest")), []byte(Digest(body))) != 1 {
			return ErrDigest
		}
	}

	signed, err := signingString(req, sig.Headers)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig.Value); err != nil {
		return ErrBadSignature
	}
	return nil
}

// signingString is what is signed: each header named, on a line of its own,
// as the draft lays it out.
func signingString(req *http.Request, headers []string) (string, error) {
	lines := make([]string, 0, len(headers))
	for _, name := range headers {
		var value string
		switch name {
		case "(request-target)":
			target := req.URL.RequestURI()
			value = strings.ToLower(req.Method) + " " + target
		case "host":
			value = req.Host
			if value == "" && req.URL != nil {
				value = req.URL.Host
			}
		default:
			values := req.Header.Values(name)
			if len(values) == 0 {
				return "", fmt.Errorf("%w: %s is signed and not sent", ErrNotCovered, name)
			}
			value = strings.Join(values, ", ")
		}
		if value == "" {
			return "", fmt.Errorf("%w: %s is empty", ErrNotCovered, name)
		}
		lines = append(lines, name+": "+value)
	}
	return strings.Join(lines, "\n"), nil
}

// splitParams splits a header's parameters at the commas that are not
// inside quotes.
func splitParams(header string) []string {
	var (
		parts  []string
		start  int
		quoted bool
	)
	for i, c := range header {
		switch c {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				parts = append(parts, header[start:i])
				start = i + 1
			}
		}
	}
	if rest := strings.TrimSpace(header[start:]); rest != "" {
		parts = append(parts, rest)
	}
	return parts
}
