package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// c.RealIP() keys the IP-side throttles and is the address recorded on every
// sign-in. These drive the extractor every service installs, because the choice of
// trust set is not obvious from the one line that expresses it, and getting
// it wrong is silent: the service works either way, it just believes the
// wrong thing.

// requestThrough builds a request as it arrives after a chain of proxies. xff
// is the X-Forwarded-For header as received and peer is the nearest proxy.
func requestThrough(xff, peer string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/password/forgot", nil)
	req.RemoteAddr = peer + ":54321"
	if xff != "" {
		req.Header.Set(echo.HeaderXForwardedFor, xff)
	}
	return req
}

func TestClientIPIsTheRightmostUntrustedAddress(t *testing.T) {
	extract := ClientIPExtractor()

	const (
		client  = "203.0.113.9"
		ingress = "10.244.1.7"
		nginx   = "10.244.2.4"
	)

	cases := []struct {
		what string
		xff  string
		peer string
		want string
	}{
		{"the client, through two private hops", client + ", " + ingress, nginx, client},
		// The attack. A client can only prepend, so its forgery always sits
		// further left than the truth, and a right-to-left walk meets the
		// truth first.
		{"a forged entry to the left of the client is ignored", "1.2.3.4, " + client + ", " + ingress, nginx, client},
		{"a forged private entry does not pass as a proxy", "10.0.0.1, " + client + ", " + ingress, nginx, client},
		{"many forged entries change nothing", "1.1.1.1, 2.2.2.2, 3.3.3.3, " + client + ", " + ingress, nginx, client},
		{"junk to the left of the client is never reached", "not-an-ip, " + client + ", " + ingress, nginx, client},
		// Junk among the trusted hops is our own misconfiguration, not
		// something a client can arrange. Echo then refuses the whole list
		// and returns the direct peer: wrong, visibly, and nobody else's.
		{"junk among the proxies falls back to the direct peer", client + ", not-an-ip", nginx, nginx},
		{"no header at all is the direct peer", "", client, client},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			if got := extract(requestThrough(c.xff, c.peer)); got != c.want {
				t.Errorf("client IP = %q, want %q (X-Forwarded-For %q, peer %s)", got, c.want, c.xff, c.peer)
			}
		})
	}
}

// Local development is unchanged. Nothing sends the header, so nginx writes one
// holding the host gateway and the peer is the nginx container. Both are
// private, every entry is trusted, and Echo falls back to the leftmost: the
// value recorded before, which the location lookup reports as local.
func TestClientIPInLocalCompose(t *testing.T) {
	extract := ClientIPExtractor()

	for _, c := range []struct{ what, xff, peer, want string }{
		{"Docker Desktop's host gateway", "192.168.65.1", "172.20.1.2", "192.168.65.1"},
		{"the compose bridge", "172.20.0.1", "172.20.1.2", "172.20.0.1"},
	} {
		t.Run(c.what, func(t *testing.T) {
			if got := extract(requestThrough(c.xff, c.peer)); got != c.want {
				t.Errorf("client IP = %q, want %q", got, c.want)
			}
		})
	}
}
