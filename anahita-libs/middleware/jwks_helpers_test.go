package middleware

import "time"

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
