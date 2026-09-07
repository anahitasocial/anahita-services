package handlers

import (
	"errors"
	"log"
	"net/http"

	"anahita-libs/sessions"
)

// requireStepUp refuses unless the person proved themselves recently at
// auth-service's /reauth/* endpoints.
//
// person-service issues no proofs. It reads a marker auth-service wrote, using
// the key format both share via anahita-libs/sessions — which is why that
// format moved out of auth-service's handlers when this was added.
//
// FAILS CLOSED, deliberately, and in two ways that look like outages:
//
//   - no session store configured (Redis unreachable at startup)
//   - the store errors on the read
//
// Both refuse the action. This is the opposite of the rate limiter's
// fail-open, and the asymmetry is the point: a limiter that cannot count
// should not lock everyone out of signing in, but a gate that cannot check
// must not wave through the one action that cannot be taken back. Failing
// closed costs somebody a retry; failing open costs somebody their profile.
func (h *Env) requireStepUp(personID int64) error {
	if h.Session == nil {
		return errors.New("step-up: no session store; refusing")
	}
	return sessions.RequireStepUp(h.Session, personID)
}

// clearStepUp consumes the marker, so one proof authorises one action.
//
// Best-effort: the action has already happened, and failing over an uncleared
// marker would report a success as a failure. Its lifetime is bounded by
// sessions.StepUpTTL regardless.
func (h *Env) clearStepUp(personID int64) {
	if err := sessions.ClearStepUp(h.Session, personID); err != nil {
		log.Printf("step-up: clear failed (person=%d): %v", personID, err)
	}
}

// stepUpStatus maps the refusal to a status code.
//
// 403 for a missing proof, because the client's job is to re-authenticate and
// retry — the same contract auth-service's own guarded endpoints use, so the
// React StepUp container needs no second case. 500 for a store failure, which
// is not something the client can fix by proving anything.
func stepUpStatus(err error) int {
	if errors.Is(err, sessions.ErrStepUpRequired) {
		return http.StatusForbidden
	}
	return http.StatusInternalServerError
}
