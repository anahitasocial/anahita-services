package handlers

import "testing"

// The step-up limiter now tells the person it tripped. That message must fire
// exactly once per window: an attacker guessing passwords would otherwise turn
// the warning into a flood aimed at the person being warned, burying it in the
// noise it generates.

func TestRateLimitCheckReportsTheCrossingOnce(t *testing.T) {
	env := newTestEnv().Env

	limit := rateLimits[rateLimitScopeReauth]

	var crossings int
	// Well past the cap, so a "first" that is really "every time after the
	// cap" shows up as a count in the dozens rather than one.
	for i := int64(1); i <= limit.max+8; i++ {
		exceeded, first := env.rateLimitCheck(rateLimitScopeReauth, "person-42")

		if i <= limit.max && exceeded {
			t.Fatalf("attempt %d of %d was rejected; the budget is spent early",
				i, limit.max)
		}
		if i > limit.max && !exceeded {
			t.Fatalf("attempt %d exceeded the cap of %d but was allowed",
				i, limit.max)
		}
		if first {
			crossings++
			if i != limit.max+1 {
				t.Errorf("reported the crossing at attempt %d; the cap is %d",
					i, limit.max)
			}
		}
	}

	if crossings != 1 {
		t.Errorf("reported the crossing %d times; one attacker would send the "+
			"owner that many emails", crossings)
	}
}

// A counter that cannot be read fails open on the limit, and a limit that did
// not trip cannot have just tripped.
func TestRateLimitCheckReportsNothingWithoutASubject(t *testing.T) {
	env := newTestEnv().Env

	exceeded, first := env.rateLimitCheck(rateLimitScopeReauth, "")
	if exceeded || first {
		t.Errorf("exceeded=%v first=%v; an empty subject is not limited and "+
			"must not notify", exceeded, first)
	}
}

// A scope with no configured limit must not notify either — otherwise a typo
// in a scope name becomes a mail trigger.
func TestRateLimitCheckReportsNothingForAnUnknownScope(t *testing.T) {
	env := newTestEnv().Env

	exceeded, first := env.rateLimitCheck(rateLimitScope("no_such_scope"), "person-42")
	if exceeded || first {
		t.Errorf("exceeded=%v first=%v for an unknown scope", exceeded, first)
	}
}
