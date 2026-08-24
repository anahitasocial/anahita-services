package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// jwksServer serves a JWKS document whose key set the test can swap at
// will, and counts how many times it was fetched.
type jwksServer struct {
	*httptest.Server
	fetches atomic.Int64
	body    atomic.Value // string
}

func newJWKSServer(t *testing.T, kids ...string) *jwksServer {
	t.Helper()

	s := &jwksServer{}
	s.setKids(t, kids...)
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.fetches.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, s.body.Load().(string))
	}))
	t.Cleanup(s.Close)
	return s
}

// setKids replaces the served key set. Keys are freshly generated, so a
// kid that disappears and returns is a genuinely different key.
func (s *jwksServer) setKids(t *testing.T, kids ...string) {
	t.Helper()

	type jwkOut struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	}
	out := struct {
		Keys []jwkOut `json:"keys"`
	}{}

	for _, kid := range kids {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate key for %q: %v", kid, err)
		}
		out.Keys = append(out.Keys, jwkOut{
			Kty: "RSA",
			Kid: kid,
			N:   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		})
	}

	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}
	s.body.Store(string(raw))
}

// withinTimeout runs fn and fails the test if it has not returned in
// time. Used to catch lock-ordering bugs, which otherwise hang the whole
// package instead of failing one test.
func withinTimeout(t *testing.T, d time.Duration, what string, fn func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()

	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not return within %s — likely deadlocked", what, d)
	}
}

// TestPublicKeyForKID_UnknownKidAfterRefreshDoesNotDeadlock covers a kid
// that is still absent once a refresh has succeeded — an access token
// signed by a key that has since been removed from the JWKS.
//
// The path used to take a read lock with a deferred unlock and then call
// markUnknownKid, which takes the write lock. sync.RWMutex is not
// upgradable, so the goroutine blocked forever, and because a blocked
// writer also blocks later readers, every subsequent token validation in
// the process wedged behind it.
func TestPublicKeyForKID_UnknownKidAfterRefreshDoesNotDeadlock(t *testing.T) {
	server := newJWKSServer(t, "kid-current")
	client := NewJWKSClient(server.URL)

	// Warm the cache so lastAttemptedAt is set, then move past the
	// refresh floor so the lookup below actually performs a fetch.
	if _, err := client.PublicKeyForKID("kid-current"); err != nil {
		t.Fatalf("warm cache: %v", err)
	}
	client.setLastAttemptedForTest(time.Now().Add(-2 * jwksRefreshFloor))

	withinTimeout(t, 5*time.Second, "PublicKeyForKID with a removed kid", func() {
		if _, err := client.PublicKeyForKID("kid-that-does-not-exist"); err == nil {
			t.Error("expected an error for a kid absent after refresh")
		}
	})

	// The client must still be usable afterwards.
	withinTimeout(t, 5*time.Second, "PublicKeyForKID after a miss", func() {
		key, err := client.PublicKeyForKID("kid-current")
		if err != nil {
			t.Errorf("client unusable after a miss: %v", err)
		}
		if key == nil {
			t.Error("expected a key for the still-published kid")
		}
	})
}

// TestPublicKeyForKID_RevokedKeyStopsVerifying is the reason a positive
// TTL exists. A cached kid used to be trusted forever, so removing a key
// from the JWKS did not stop this process accepting tokens signed with
// it until some unrelated unknown kid happened to force a refetch.
func TestPublicKeyForKID_RevokedKeyStopsVerifying(t *testing.T) {
	server := newJWKSServer(t, "kid-a", "kid-b")
	client := NewJWKSClient(server.URL)

	if _, err := client.PublicKeyForKID("kid-a"); err != nil {
		t.Fatalf("warm cache: %v", err)
	}

	// Revoke kid-a upstream.
	server.setKids(t, "kid-b")

	// Still inside the max age: the cached key is served, no refetch.
	key, err := client.PublicKeyForKID("kid-a")
	if err != nil {
		t.Fatalf("inside max age the cached key must still be served: %v", err)
	}
	if key == nil {
		t.Fatal("inside max age the cached key must still be served")
	}

	// Age the cache past the TTL.
	client.setLastFetchedForTest(time.Now().Add(-2 * jwksMaxAge))
	client.setLastAttemptedForTest(time.Now().Add(-2 * jwksMaxAge))

	if _, err := client.PublicKeyForKID("kid-a"); err == nil {
		t.Error("a key removed from the JWKS must stop verifying once the cache ages out")
	}

	key, err = client.PublicKeyForKID("kid-b")
	if err != nil {
		t.Errorf("keys still in the JWKS must keep working: %v", err)
	}
	if key == nil {
		t.Error("keys still in the JWKS must keep working")
	}
}

// TestPublicKeyForKID_ServesStaleKeysWhenJWKSIsDown is the other half of
// the TTL. Every service in the fleet verifies tokens through this
// client, so an unreachable JWKS endpoint must not become a fleet-wide
// outage: signatures are still valid and access tokens are short-lived.
func TestPublicKeyForKID_ServesStaleKeysWhenJWKSIsDown(t *testing.T) {
	server := newJWKSServer(t, "kid-a")
	client := NewJWKSClient(server.URL)

	if _, err := client.PublicKeyForKID("kid-a"); err != nil {
		t.Fatalf("warm cache: %v", err)
	}

	server.Close()

	client.setLastFetchedForTest(time.Now().Add(-2 * jwksMaxAge))
	client.setLastAttemptedForTest(time.Now().Add(-2 * jwksMaxAge))

	key, err := client.PublicKeyForKID("kid-a")
	if err != nil {
		t.Errorf("a JWKS outage must not stop verification of an already-known kid: %v", err)
	}
	if key == nil {
		t.Error("a JWKS outage must not stop verification of an already-known kid")
	}
}

// TestPublicKeyForKID_FreshCacheDoesNotRefetch guards the hot path. This
// runs on every authenticated request across every service, so a stale
// check that refetched each time would put the JWKS endpoint under the
// full request load of the fleet.
func TestPublicKeyForKID_FreshCacheDoesNotRefetch(t *testing.T) {
	server := newJWKSServer(t, "kid-a")
	client := NewJWKSClient(server.URL)

	if _, err := client.PublicKeyForKID("kid-a"); err != nil {
		t.Fatalf("warm cache: %v", err)
	}
	after := server.fetches.Load()

	for i := 0; i < 50; i++ {
		if _, err := client.PublicKeyForKID("kid-a"); err != nil {
			t.Fatalf("warm lookup %d: %v", i, err)
		}
	}

	if got := server.fetches.Load(); got != after {
		t.Errorf("a warm cache must not hit the network: fetches went %d -> %d", after, got)
	}
}

// TestPublicKeyForKID_FailedFetchBacksOff covers the failure path of the
// refresh floor. lastFetchedAt is only advanced on success, so keying the
// floor off it meant a down JWKS endpoint was retried on every single
// request, each costing a connection attempt against the fetch timeout.
func TestPublicKeyForKID_FailedFetchBacksOff(t *testing.T) {
	server := newJWKSServer(t, "kid-a")
	client := NewJWKSClient(server.URL)

	if _, err := client.PublicKeyForKID("kid-a"); err != nil {
		t.Fatalf("warm cache: %v", err)
	}

	server.Close()
	client.setLastAttemptedForTest(time.Now().Add(-2 * jwksRefreshFloor))

	// First miss attempts a fetch and fails.
	if _, err := client.PublicKeyForKID("unknown-1"); err == nil {
		t.Fatal("expected an error for an unknown kid with the endpoint down")
	}
	fetchesAfterFirst := server.fetches.Load()

	// Subsequent misses inside the floor must not attempt another.
	for i := 0; i < 10; i++ {
		if _, err := client.PublicKeyForKID(fmt.Sprintf("unknown-%d", i+2)); err == nil {
			t.Fatalf("expected an error for unknown-%d", i+2)
		}
	}

	if got := server.fetches.Load(); got != fetchesAfterFirst {
		t.Errorf("a failed fetch must back off rather than retry on every request: fetches went %d -> %d",
			fetchesAfterFirst, got)
	}
}

// TestPublicKeyForKID_EmptyKid pins the cheapest rejection.
func TestPublicKeyForKID_EmptyKid(t *testing.T) {
	client := NewJWKSClient("http://127.0.0.1:1/unused")
	if _, err := client.PublicKeyForKID(""); err == nil {
		t.Error("expected an error for an empty kid")
	}
}
