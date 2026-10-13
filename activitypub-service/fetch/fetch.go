// Package fetch reads from other servers of the fediverse: who somebody is,
// and where a handle points.
//
// It goes where it is told by people and by other servers, so it goes
// through the guarded client (anahita-libs/safehttp): to public addresses
// only, over https only, and it reads no more than a description needs.
// What it asks for it signs, as this installation: many servers answer
// nobody who does not say who is asking.
package fetch

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"activitypub/httpsig"

	"anahita-libs/safehttp"
)

const (
	Timeout      = 10 * time.Second
	MaxRedirects = 3
	// The most of an answer that is read. A person's description is a few
	// kilobytes.
	MaxBytes = 1 << 20

	accept = `application/activity+json, application/ld+json; profile="https://www.w3.org/ns/activitystreams"`
)

var (
	// ErrGone is the other server saying there is nothing at an address,
	// or that there was and is no more.
	ErrGone = errors.New("nothing is there")
	// ErrNotAllowed is an address that is not gone to.
	ErrNotAllowed = safehttp.ErrNotAllowed
	// ErrUnreadable is an answer that is not what was asked for.
	ErrUnreadable = errors.New("the answer could not be read")
)

// Signer is who a fetch is signed as: the name of a key, and the key.
type Signer func() (keyID string, key *rsa.PrivateKey, err error)

// Client reads from other servers.
type Client struct {
	http      *http.Client
	signer    Signer
	userAgent string
	now       func() time.Time
}

// New is a client that goes only to public addresses.
func New(userAgent string, signer Signer) *Client {
	return NewWith(userAgent, signer, safehttp.PublicAddress)
}

// NewWith is a client that goes where `allow` lets it. For the lab, where
// every server is at a private address, and for tests.
func NewWith(userAgent string, signer Signer, allow safehttp.Allow) *Client {
	return &Client{
		// Redirects are followed here and not by the client, so that each
		// hop is checked and signed afresh.
		http:      safehttp.NewClient(safehttp.Options{Allow: allow, Timeout: Timeout, MaxRedirects: 0}),
		signer:    signer,
		userAgent: userAgent,
		now:       time.Now,
	}
}

// newFrom is a client over an http.Client that is given. For tests, which
// need one that trusts the test's own certificate.
func newFrom(client *http.Client, signer Signer) *Client {
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{http: client, signer: signer, userAgent: "test", now: time.Now}
}

// LabAddresses allows any address on the web's secure port. For the
// federation lab only: there every server is at a private address.
func LabAddresses(ip net.IP, port string) bool {
	return ip != nil && port == "443"
}

// Object fetches something another server has, as ActivityPub describes it,
// into `into`. It answers with the address the answer finally came from,
// which is the one to compare with what the answer says it is.
func (c *Client) Object(ctx context.Context, address string, into any) (string, error) {
	return c.get(ctx, address, accept, true, into)
}

// JSON fetches plain JSON, unsigned: a WebFinger answer.
func (c *Client) JSON(ctx context.Context, address string, into any) (string, error) {
	return c.get(ctx, address, "application/jrd+json, application/json", false, into)
}

// Bytes fetches a file from another server: somebody's picture. Unsigned,
// https only, to public addresses only, following where it is sent a few
// times, and never more than `max` bytes. It answers with what kind of file
// the server said it is, which is not to be believed without looking.
func (c *Client) Bytes(ctx context.Context, address string, max int64) ([]byte, string, error) {
	for hop := 0; hop <= MaxRedirects; hop++ {
		parsed, err := url.Parse(address)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return nil, "", fmt.Errorf("%w: %s", ErrNotAllowed, address)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrNotAllowed, err)
		}
		req.Header.Set("User-Agent", c.userAgent)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, "", err
		}

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			next, err := resp.Location()
			resp.Body.Close()
			if err != nil {
				return nil, "", fmt.Errorf("%w: sent on to nowhere", ErrUnreadable)
			}
			address = next.String()
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("%w: status %d from %s", ErrUnreadable, resp.StatusCode, parsed.Host)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
		if err != nil {
			return nil, "", err
		}
		if int64(len(body)) > max {
			return nil, "", fmt.Errorf("%w: more than %d bytes", ErrUnreadable, max)
		}
		return body, resp.Header.Get("Content-Type"), nil
	}
	return nil, "", fmt.Errorf("%w: sent on too many times", ErrUnreadable)
}

// ErrRefused is the other server saying it will never take what was sent:
// trying again would get the same answer.
var ErrRefused = errors.New("the other server will not take it")

// Post sends an activity to an inbox, signed by whoever it is sent for.
//
// Nothing is followed: an inbox that sends the post on elsewhere is not
// written to. What was signed was signed for one address.
func (c *Client) Post(ctx context.Context, inbox string, body []byte, signer Signer) error {
	parsed, err := url.Parse(inbox)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("%w: %s", ErrNotAllowed, inbox)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, inbox, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotAllowed, err)
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("User-Agent", c.userAgent)

	keyID, key, err := signer()
	if err != nil {
		return err
	}
	if err := httpsig.Sign(req, body, keyID, key, c.now()); err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	// Wrong in a way that will be as wrong tomorrow. Not 401 or 403: a
	// server that could not fetch our key this minute may the next.
	case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity ||
		resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusRequestEntityTooLarge:
		return fmt.Errorf("%w: status %d from %s", ErrRefused, resp.StatusCode, parsed.Host)
	}
	return fmt.Errorf("status %d from %s", resp.StatusCode, parsed.Host)
}

func (c *Client) get(ctx context.Context, address, accepts string, sign bool, into any) (string, error) {
	for hop := 0; hop <= MaxRedirects; hop++ {
		parsed, err := url.Parse(address)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return "", fmt.Errorf("%w: %s", ErrNotAllowed, address)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrNotAllowed, err)
		}
		req.Header.Set("Accept", accepts)
		req.Header.Set("User-Agent", c.userAgent)

		if sign && c.signer != nil {
			keyID, key, err := c.signer()
			if err != nil {
				return "", err
			}
			if err := httpsig.Sign(req, nil, keyID, key, c.now()); err != nil {
				return "", err
			}
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return "", err
		}

		switch {
		case resp.StatusCode >= 300 && resp.StatusCode < 400:
			next, err := resp.Location()
			resp.Body.Close()
			if err != nil {
				return "", fmt.Errorf("%w: sent on to nowhere", ErrUnreadable)
			}
			address = next.String()
			continue

		case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
			resp.Body.Close()
			return "", ErrGone

		case resp.StatusCode != http.StatusOK:
			resp.Body.Close()
			return "", fmt.Errorf("%w: status %d from %s", ErrUnreadable, resp.StatusCode, parsed.Host)
		}

		defer resp.Body.Close()

		kind := strings.ToLower(resp.Header.Get("Content-Type"))
		if !strings.Contains(kind, "json") {
			return "", fmt.Errorf("%w: %s sent %q", ErrUnreadable, parsed.Host, kind)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
		if err != nil {
			return "", err
		}
		if len(body) > MaxBytes {
			return "", fmt.Errorf("%w: more than %d bytes", ErrUnreadable, MaxBytes)
		}
		if err := json.Unmarshal(body, into); err != nil {
			return "", fmt.Errorf("%w: %v", ErrUnreadable, err)
		}
		return address, nil
	}

	return "", fmt.Errorf("%w: sent on too many times", ErrUnreadable)
}
