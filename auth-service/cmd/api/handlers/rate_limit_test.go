package handlers

import (
	"errors"
	"strings"
	"testing"
)

// The limiter guards every credential-grade path in the service, and two of
// its properties are decisions rather than mechanics — it COUNTS on every
// call, and it fails OPEN. Both are the kind of thing a later refactor
// "tidies" without realising what it changed, so both are pinned here.

func TestRateLimitAllowsExactlyTheBudget(t *testing.T) {
	env := newTestEnv().Env

	// pw_edit_person: 5 per 15 minutes.
	for i := range 5 {
		if env.rateLimitExceeded(rateLimitScopePasswordEdit, "person-42") {
			t.Fatalf("attempt %d was refused inside the budget of 5", i+1)
		}
	}
	if !env.rateLimitExceeded(rateLimitScopePasswordEdit, "person-42") {
		t.Error("the sixth attempt was allowed")
	}
}

// Scopes and subjects both partition the counters. A shared counter would
// let one person's failures lock out another's, or one flow's failures
// spend a different flow's budget.
func TestRateLimitCountersAreIndependent(t *testing.T) {
	env := newTestEnv().Env

	for range 5 {
		env.rateLimitExceeded(rateLimitScopePasswordEdit, "person-42")
	}
	if !env.rateLimitExceeded(rateLimitScopePasswordEdit, "person-42") {
		t.Fatal("budget was not exhausted")
	}

	if env.rateLimitExceeded(rateLimitScopePasswordEdit, "person-99") {
		t.Error("exhausting one subject's budget refused another's first attempt")
	}
	if env.rateLimitExceeded(rateLimitScopeReauth, "person-42") {
		t.Error("exhausting one scope's budget refused another scope's first attempt")
	}
}

func TestRateLimitResetRefillsTheBudget(t *testing.T) {
	env := newTestEnv().Env

	for range 6 {
		env.rateLimitExceeded(rateLimitScopePasswordEdit, "person-42")
	}
	if !env.rateLimitExceeded(rateLimitScopePasswordEdit, "person-42") {
		t.Fatal("budget was not exhausted")
	}

	env.rateLimitReset(rateLimitScopePasswordEdit, "person-42")

	if env.rateLimitExceeded(rateLimitScopePasswordEdit, "person-42") {
		t.Error("the counter survived a reset")
	}
}

// Deliberate, and the uncomfortable half of the design: a store that is
// down must not take account recovery down with it. The exposure while
// Redis is unavailable is the exposure the service had before these
// counters existed.
func TestRateLimitFailsOpenWhenTheStoreIsDown(t *testing.T) {
	te := newTestEnv()
	te.Sess.incrErr = errors.New("redis is down")

	for i := range 20 {
		if te.Env.rateLimitExceeded(rateLimitScopePasswordEdit, "person-42") {
			t.Fatalf("attempt %d was refused while the store was down", i+1)
		}
	}
}

// An empty subject means the caller had nothing to key on — no client IP,
// say. Counting those together would let anonymous traffic exhaust one
// shared budget on everybody's behalf.
func TestRateLimitIgnoresAnEmptySubject(t *testing.T) {
	te := newTestEnv()

	for range 20 {
		if te.Env.rateLimitExceeded(rateLimitScopePasswordEdit, "") {
			t.Fatal("an empty subject was rate limited")
		}
	}
	if len(te.Sess.counters) != 0 {
		t.Errorf("an empty subject wrote %d counter(s)", len(te.Sess.counters))
	}
}

// A scope with no budget declared is a programming error, and the choice
// made is to log and allow rather than to refuse everything — refusing
// would take out whichever flow the typo is in.
func TestRateLimitAllowsAnUnknownScope(t *testing.T) {
	env := newTestEnv().Env

	for range 20 {
		if env.rateLimitExceeded(rateLimitScope("no_such_scope"), "person-42") {
			t.Fatal("an unknown scope was rate limited")
		}
	}
}

// The subject is hashed into the key. An email address or a live reset
// token sitting in a Redis key is a copy of that value in a second system
// with a different retention story.
func TestRateLimitKeysDoNotCarryTheSubject(t *testing.T) {
	const email = "ada@example.com"

	key := rateLimitKey(rateLimitScopePasswordEmail, email)

	if key == "" {
		t.Fatal("empty key")
	}
	if strings.Contains(key, email) {
		t.Errorf("key %q carries the subject in the clear", key)
	}
	// The scope stays legible, so a key can still be read in Redis.
	if !strings.Contains(key, string(rateLimitScopePasswordEmail)) {
		t.Errorf("key %q does not name its scope", key)
	}
	if rateLimitKey(rateLimitScopePasswordEmail, "other@example.com") == key {
		t.Error("two subjects produced the same key")
	}
}
