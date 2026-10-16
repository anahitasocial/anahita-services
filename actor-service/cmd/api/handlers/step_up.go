package handlers

import (
	"anahita-libs/responses"
	"errors"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"

	"anahita-libs/sessions"
)

// requireStepUp refuses unless the person proved themselves recently at
// auth-service's /reauth/* endpoints.
//
// This service issues no proofs. It reads a marker auth-service wrote, using
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
// closed costs somebody a retry; failing open costs somebody their profile,
// or a group everything that was posted into it.
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

// stepUpRefusal renders the refusal, with a body that says which kind it is.
//
// 403 for a missing proof, matching auth-service's guarded endpoints so the
// React StepUp container needs no second case. 500 for a store failure, which
// the client cannot fix by proving anything.
//
// THE BODY IS NOT DECORATION. Here, unlike in auth-service, 403 already meant
// something else: CanDelete refusing somebody who may not delete this profile
// at all. The React card opens the step-up dialog on a 403, so without a way
// to tell the two apart, a person who simply lacks permission would prove
// themselves, retry, be refused again, and be shown the dialog again — a loop
// with no exit. "step_up_required" is what breaks it.
func stepUpRefusal(c echo.Context, err error) error {
	if errors.Is(err, sessions.ErrStepUpRequired) {
		return c.JSON(http.StatusForbidden, responses.Refusal{Error: "step_up_required"})
	}
	return c.NoContent(http.StatusInternalServerError)
}
