// Package ratelimit is a fixed-window counter: at most Max of something per
// Window, per subject, counted in Redis.
//
// It began in auth-service, guarding sign-in and the forms around it, and
// lives here so that any service can cap an action the same way. The scope
// names what is being counted and the subject names who or what it is counted
// against: an account, an address, a network address.
//
// Three properties every caller relies on:
//
//   - The subject is hashed into the key, so an email address or an IP is
//     never a Redis key.
//   - It fails OPEN. If the counter cannot be reached, the action is allowed
//     and the failure is logged. A limiter that locks everybody out when
//     Redis blinks is a worse outage than the abuse it prevents.
//   - An empty subject is never limited, and never counted.
package ratelimit

import (
	"fmt"
	"log"
	"time"

	"anahita-libs/sessions"
)

// Limit is how many are allowed in how long.
type Limit struct {
	Max    int64
	Window time.Duration
}

// Counter is the part of the session store this needs. sessions.SessionInterface
// satisfies it.
type Counter interface {
	Incr(key string, ttl time.Duration) (int64, error)
	Del(keys ...string) error
}

// Key is where one subject's count for one scope is kept.
func Key(scope, subject string) string {
	return fmt.Sprintf("ratelimit:%s:%s", scope, sessions.HashString(subject))
}

// Check counts one more and reports whether the subject is now over the limit.
//
// `first` is true only for the attempt that crossed it, which is the one
// moment a caller might want to tell somebody, once, that they are locked out.
func Check(store Counter, scope, subject string, limit Limit) (exceeded, first bool) {
	if subject == "" {
		return false, false
	}

	count, err := store.Incr(Key(scope, subject), limit.Window)
	if err != nil {
		log.Printf("rate limit: counter failed for scope %s, allowing: %v", scope, err)
		return false, false
	}

	if count > limit.Max {
		log.Printf("rate limit: scope %s exceeded (%d/%d)", scope, count, limit.Max)
		return true, count == limit.Max+1
	}

	return false, false
}

// Exceeded is Check for callers that do not need to know which attempt
// crossed the limit.
func Exceeded(store Counter, scope, subject string, limit Limit) bool {
	exceeded, _ := Check(store, scope, subject, limit)
	return exceeded
}

// Reset clears a subject's count, for a caller whose action succeeded and
// whose earlier attempts should no longer be held against it.
func Reset(store Counter, scope, subject string) {
	if subject == "" {
		return
	}

	if err := store.Del(Key(scope, subject)); err != nil {
		log.Printf("rate limit: reset failed for scope %s: %v", scope, err)
	}
}
