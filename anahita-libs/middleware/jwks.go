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

	mu              sync.RWMutex
	keys            map[string]*rsa.PublicKey
	unknownKids     map[string]time.Time
	lastFetchedAt   time.Time
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
// fetching JWKS if the kid is unknown. Returns an error if the kid
// cannot be resolved after a fetch attempt.
func (c *JWKSClient) PublicKeyForKID(kid string) (*rsa.PublicKey, error) {
	if kid == "" {
		return nil, errors.New("jwks: empty kid")
	}

	// Fast path: read lock, hit cache.
	c.mu.RLock()
	if key, ok := c.keys[kid]; ok {
		c.mu.RUnlock()
		return key, nil
	}
	if firstSeen, ok := c.unknownKids[kid]; ok && time.Since(firstSeen) < jwksUnknownKidTTL {
		c.mu.RUnlock()
		return nil, fmt.Errorf("jwks: unknown kid %q (negative-cached)", kid)
	}
	lastFetch := c.lastFetchedAt
	c.mu.RUnlock()

	// Slow path: refresh if we're past the floor.
	if time.Since(lastFetch) < jwksRefreshFloor {
		c.markUnknownKid(kid)
		return nil, fmt.Errorf("jwks: unknown kid %q (refresh floor not elapsed)", kid)
	}

	if err := c.refresh(); err != nil {
		return nil, fmt.Errorf("jwks: refresh failed: %w", err)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if key, ok := c.keys[kid]; ok {
		return key, nil
	}

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

// refresh fetches the JWKS endpoint and replaces the cache. Concurrent
// callers wait on the same lock; one wins and the rest see fresh state.
func (c *JWKSClient) refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check the floor under the write lock — a sibling call may have
	// just refreshed.
	if time.Since(c.lastFetchedAt) < jwksRefreshFloor && c.lastFetchFailed == nil {
		return nil
	}

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
