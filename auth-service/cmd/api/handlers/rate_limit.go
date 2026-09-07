package handlers

import (
	"fmt"
	"log"
	"time"

	"anahita-libs/sessions"
)

// ---------------------------------------------------------------------
// Rate limiting
// ---------------------------------------------------------------------
//
// A fixed-window counter per (scope, subject), kept in the session
// store. Scopes are declared here rather than passed as strings at the
// call site so that the limit and the window travel with the name —
// a caller cannot accidentally apply the reset budget to the forgot
// flow, and reading rateLimits below tells you every limit the service
// enforces.
//
// Subjects are hashed into the key. Two reasons: an email address or a
// reset token in a Redis key is a copy of that value sitting in a
// second system with a different retention story, and subjects are
// caller-supplied and unbounded, so hashing is also what keeps key
// length fixed.

type rateLimitScope string

const (
	// rateLimitScopePasswordEmail caps reset emails per address.
	// Without it, POST /password/forgot is an unauthenticated way to
	// send mail to any address on demand.
	rateLimitScopePasswordEmail rateLimitScope = "pw_forgot_email"

	// rateLimitScopePasswordIP caps reset requests per source address.
	// Catches the case the per-email counter cannot: spraying MANY
	// addresses passes every fresh email check, so something has to
	// count the sprayer rather than the targets.
	rateLimitScopePasswordIP rateLimitScope = "pw_forgot_ip"

	// rateLimitScopePasswordReset caps "is this the current password?"
	// probes against one reset link. Keyed on the token, so a fresh
	// link starts with a fresh budget — see the long note in
	// passwordHandler.Reset for why that distinction matters.
	rateLimitScopePasswordReset rateLimitScope = "pw_reset_token"

	// rateLimitScopePasswordEdit caps password changes per person on
	// the authenticated endpoint.
	rateLimitScopePasswordEdit rateLimitScope = "pw_edit_person"

	// rateLimitScopeReauth caps step-up attempts per person.
	//
	// /reauth/password compares a submitted password against the
	// stored hash and says whether it matched. That is a password
	// oracle for anyone holding a session, which is the one thing a
	// step-up gate cannot assume its caller does not have — a stolen
	// unlocked browser is a session. Unmetered, it would let that
	// attacker walk off with the plaintext to try elsewhere, which is
	// worth more to them than the account they already hold.
	rateLimitScopeReauth rateLimitScope = "reauth_person"
)

type rateLimit struct {
	max    int64
	window time.Duration
}

// rateLimits holds the budget for every scope.
//
// The windows are deliberately uneven. The forgot-password counters
// guard an outbound mail path, where the cost of a false positive is
// low and the abuse is slow and wide, so they run for an hour. The two
// password counters guard a bcrypt comparison that answers a yes/no
// question about a credential, so they run for as long as the thing
// they protect stays useful — the reset window is the token's own
// lifetime, and there is no point letting a probe budget outlive the
// link it is spent on.
var rateLimits = map[rateLimitScope]rateLimit{
	rateLimitScopePasswordEmail: {max: 3, window: time.Hour},
	rateLimitScopePasswordIP:    {max: 10, window: time.Hour},
	rateLimitScopePasswordReset: {max: 5, window: 15 * time.Minute},
	rateLimitScopePasswordEdit:  {max: 5, window: 15 * time.Minute},
	rateLimitScopeReauth:        {max: 5, window: 15 * time.Minute},
}

func rateLimitKey(scope rateLimitScope, subject string) string {
	return fmt.Sprintf("ratelimit:%s:%s", scope, sessions.HashString(subject))
}

// rateLimitExceeded counts one attempt against a scope and reports
// whether the caller is now over budget.
//
// It COUNTS. Every call is an attempt spent, so it belongs on the path
// where the attempt actually happens and nowhere else — calling it to
// "check" without intending to consume quietly halves the budget.
//
// An empty subject is not counted and never limited: it means the
// caller had nothing to key on (no client IP, say), and lumping every
// such request into one shared counter would let anonymous traffic
// exhaust a budget on everyone's behalf.
//
// Fails OPEN. A store that is down, or the cookie backend that has
// nowhere to count, both report "not exceeded" and log. This is the
// uncomfortable half of the design and it is chosen rather than
// overlooked: failing closed turns a Redis outage into a service where
// nobody can reset a password or change one, and account recovery is
// exactly the thing that must keep working on a bad day. The exposure
// while the store is down is the exposure this service had before
// these counters existed.
func (env *Env) rateLimitExceeded(scope rateLimitScope, subject string) bool {
	exceeded, _ := env.rateLimitCheck(scope, subject)
	return exceeded
}

// rateLimitCheck is rateLimitExceeded plus whether this call is the one that
// crossed the line.
//
// The second return exists for the handlers that want to TELL somebody a limit
// tripped. Mailing on every rejected attempt would turn a guessing attack into
// a mail flood aimed at the person being attacked — burying the warning in the
// noise it generates, and making the notification the more effective attack of
// the two. Notifying only on the first crossing gives exactly one message per
// window.
//
// "First" is read off the counter rather than kept in a second key: the count
// passes through limit.max exactly once per window, because the window's TTL
// is set on the first increment and the counter only rises.
//
// A counter failure reports neither exceeded nor first. Failing open on the
// limit is the existing behaviour — an outage in the session store should not
// lock everyone out of signing in — and it follows that a limit which did not
// trip cannot have just tripped.
func (env *Env) rateLimitCheck(scope rateLimitScope, subject string) (exceeded, first bool) {
	if subject == "" {
		return false, false
	}

	limit, ok := rateLimits[scope]
	if !ok {
		log.Printf("rate limit: unknown scope %q, not limiting", scope)
		return false, false
	}

	count, err := env.Session.Incr(rateLimitKey(scope, subject), limit.window)
	if err != nil {
		log.Printf("rate limit: counter failed for scope %s, allowing: %v",
			scope, err)
		return false, false
	}

	if count > limit.max {
		log.Printf("rate limit: scope %s exceeded (%d/%d)",
			scope, count, limit.max)
		return true, count == limit.max+1
	}

	return false, false
}

// rateLimitReset drops a counter.
//
// For budgets that exist to bound failed attempts: once the thing the
// attempts were aiming at has succeeded, the failures before it are no
// longer evidence of anything. Best-effort — a counter that survives
// costs the person a wait, not access.
func (env *Env) rateLimitReset(scope rateLimitScope, subject string) {
	if subject == "" {
		return
	}

	if err := env.Session.Del(rateLimitKey(scope, subject)); err != nil {
		log.Printf("rate limit: reset failed for scope %s: %v", scope, err)
	}
}
