package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// jwksRefreshFloor caps how often the JWKS endpoint can be re-fetched.
// Protects against attackers DoS-ing JWKS by sending tokens with random
// kid headers. Within this window, an unknown kid yields a verification
// failure without triggering a fetch.
const jwksRefreshFloor = 10 * time.Second

// jwksUnknownKidTTL is how long an unknown kid stays in the negative
// cache before a fresh fetch attempt becomes possible. Bounds the impact
// of bad-kid token floods.
const jwksUnknownKidTTL = 30 * time.Second

// jwksFetchTimeout caps how long a JWKS fetch may take.
const jwksFetchTimeout = 5 * time.Second

// jwksMaxAge is how long a successfully fetched key set stays
// authoritative before it is re-fetched.
//
// Without it, a kid that had been cached was trusted for the lifetime of
// the process: removing a key from the JWKS did not stop a running
// service accepting tokens signed with it, because a refetch only ever
// happened when some *unknown* kid arrived. Rotation usually hides this
// — the new kid forces a refresh that replaces the whole set — but
// revoking a key without immediately signing with a new one left the old
// key trusted indefinitely.
//
// Chosen to match the 15 minute access token lifetime: a revoked key
// stops verifying within roughly the time its outstanding tokens would
// have expired anyway.
const jwksMaxAge = 15 * time.Minute

// JWKSClient fetches and caches JSON Web Keys from auth-service for
// downstream microservices to verify access tokens locally.
//
// Cache strategy:
//   - First request triggers a fetch.
//   - Subsequent requests with a known kid hit the warm cache.
//   - A kid not in the cache triggers a refetch, subject to a refresh
//     floor (see jwksRefreshFloor).
//   - Unknown kids are negatively cached briefly to bound DoS impact.
//
// JWKSClient is safe for concurrent use.
type JWKSClient struct {
	url        string
	httpClient *http.Client

	mu          sync.RWMutex
	keys        map[string]*rsa.PublicKey
	unknownKids map[string]time.Time

	// lastFetchedAt is the last *successful* fetch, and is what the max
	// age is measured against.
	lastFetchedAt time.Time

	// lastAttemptedAt is the last fetch attempt, successful or not, and
	// is what the refresh floor is measured against. These have to be
	// separate: keying the floor off lastFetchedAt meant a failing JWKS
	// endpoint was never considered recently-attempted, so every request
	// launched its own fetch and paid the full timeout.
	lastAttemptedAt time.Time

	lastFetchFailed error
}

// NewJWKSClient constructs a client pointing at the given JWKS URL
// (typically auth-service's /.well-known/jwks.json).
func NewJWKSClient(jwksURL string) *JWKSClient {
	return &JWKSClient{
		url:         jwksURL,
		httpClient:  &http.Client{Timeout: jwksFetchTimeout},
		keys:        make(map[string]*rsa.PublicKey),
		unknownKids: make(map[string]time.Time),
	}
}

// PublicKeyForKID returns the cached public key for the given kid,
// fetching JWKS if the kid is unknown or the cached set has aged out.
// Returns an error if the kid cannot be resolved after a fetch attempt.
func (c *JWKSClient) PublicKeyForKID(kid string) (*rsa.PublicKey, error) {
	if kid == "" {
		return nil, errors.New("jwks: empty kid")
	}

	c.mu.RLock()
	key, known := c.keys[kid]
	negativeCached := false
	if firstSeen, ok := c.unknownKids[kid]; ok && time.Since(firstSeen) < jwksUnknownKidTTL {
		negativeCached = true
	}
	stale := time.Since(c.lastFetchedAt) >= jwksMaxAge
	lastAttempt := c.lastAttemptedAt
	c.mu.RUnlock()

	// Fast path: known kid, cache still within its max age. This runs on
	// every authenticated request in every service, so it must stay a
	// read-lock map lookup and nothing more.
	if known && !stale {
		return key, nil
	}

	if known && stale {
		// Re-fetch so that keys removed upstream stop verifying. A
		// failure here must NOT reject the request: every service in the
		// fleet validates through this client, so treating an
		// unreachable JWKS endpoint as an auth failure would turn one
		// endpoint's outage into a fleet-wide one. The signature we hold
		// is still valid and access tokens are short-lived, so serving
		// the stale key is the safer trade.
		if err := c.refresh(); err != nil {
			return key, nil
		}

		c.mu.RLock()
		refreshed, stillPresent := c.keys[kid]
		c.mu.RUnlock()

		if stillPresent {
			return refreshed, nil
		}
		return nil, fmt.Errorf("jwks: kid %q no longer present in JWKS", kid)
	}

	if negativeCached {
		return nil, fmt.Errorf("jwks: unknown kid %q (negative-cached)", kid)
	}

	// Unknown kid. Refresh if we are past the floor, which is measured
	// against the last attempt so a failing endpoint backs off too.
	if time.Since(lastAttempt) < jwksRefreshFloor {
		c.markUnknownKid(kid)
		return nil, fmt.Errorf("jwks: unknown kid %q (refresh floor not elapsed)", kid)
	}

	if err := c.refresh(); err != nil {
		c.markUnknownKid(kid)
		return nil, fmt.Errorf("jwks: refresh failed: %w", err)
	}

	c.mu.RLock()
	fetched, ok := c.keys[kid]
	c.mu.RUnlock()

	if ok {
		return fetched, nil
	}

	// markUnknownKid takes the write lock, so the read lock above must be
	// released before calling it. This previously ran under a deferred
	// RUnlock: sync.RWMutex is not upgradable, so the goroutine blocked
	// forever, and a blocked writer also stalls every later reader, which
	// wedged token validation process-wide.
	c.markUnknownKid(kid)
	return nil, fmt.Errorf("jwks: kid %q not present after refresh", kid)
}

// markUnknownKid records a kid in the negative cache. Caller must NOT
// hold the lock.
func (c *JWKSClient) markUnknownKid(kid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unknownKids[kid] = time.Now()
}

// setLastFetchedForTest backdates the last successful fetch so tests can
// age the cache past jwksMaxAge without sleeping.
func (c *JWKSClient) setLastFetchedForTest(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastFetchedAt = t
}

// setLastAttemptedForTest backdates the last fetch attempt so tests can
// step over the refresh floor without sleeping.
func (c *JWKSClient) setLastAttemptedForTest(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastAttemptedAt = t
	c.lastFetchFailed = nil
}

// refresh fetches the JWKS endpoint and replaces the cache. Concurrent
// callers wait on the same lock; one wins and the rest see fresh state.
func (c *JWKSClient) refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check the floor under the write lock — a sibling call may have
	// just refreshed. Measured against the last attempt, so a run of
	// failures backs off instead of every caller retrying immediately.
	if time.Since(c.lastAttemptedAt) < jwksRefreshFloor {
		return c.lastFetchFailed
	}
	c.lastAttemptedAt = time.Now()

	resp, err := c.httpClient.Get(c.url)
	if err != nil {
		c.lastFetchFailed = err
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.lastFetchFailed = fmt.Errorf("status %d", resp.StatusCode)
		return c.lastFetchFailed
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.lastFetchFailed = err
		return err
	}

	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		c.lastFetchFailed = err
		return fmt.Errorf("jwks: parse: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pub, err := jwkToRSAPublicKey(k)
		if err != nil {
			// Skip malformed keys; one bad key shouldn't break the set.
			continue
		}
		newKeys[k.Kid] = pub
	}

	c.keys = newKeys
	c.unknownKids = make(map[string]time.Time)
	c.lastFetchedAt = time.Now()
	c.lastFetchFailed = nil
	return nil
}

// jwksDocument is the minimal shape of a JWKS document we care about.
type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwkToRSAPublicKey converts a JWK (RSA) into a Go *rsa.PublicKey.
func jwkToRSAPublicKey(k jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() {
		return nil, errors.New("exponent overflow")
	}

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}
