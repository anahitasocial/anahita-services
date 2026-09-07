package sessions

import (
	"errors"
	"fmt"
	"time"
)

// Step-up re-authentication, shared across services.
//
// A step-up marker says "this person proved a passkey, or their password plus
// a passcode, within the last few minutes". auth-service writes it from
// /reauth/*; any service guarding a credential-grade action reads it.
//
// The key format lives here rather than in auth-service because it is now a
// contract between services. person-service and group-service gate profile
// deletion on a marker auth-service wrote, and a prefix that only one of them
// knows is a prefix the others can silently disagree with — the failure being
// that the guard never finds a marker and every deletion is refused, or worse,
// that it looks in the wrong place and finds nothing when something is there.

// KeyValueInterface is the slice of a session store that step-up needs: two
// key operations and nothing about cookies, requests or encryption.
//
// Narrow on purpose. person-service and group-service read this marker but
// issue no proofs, set no cookies and serve no session — depending on the full
// SessionInterface would have made them satisfy GetDomainName and
// GetSecureCookieKey to reach two Redis calls that use neither. SessionInterface
// satisfies this, so auth-service passes what it already has.
type KeyValueInterface interface {
	Exists(key string) (bool, error)
	Del(keys ...string) error
}

// StepUpTTL is how long one proof stays good.
//
// Long enough to fill in a form, short enough that a walk-away does not leave
// the next person holding an authorisation.
const StepUpTTL = 5 * time.Minute

const stepUpPrefix = "stepup:"

// ErrStepUpRequired means no live proof was found. Callers map it to a status
// the client understands — 403, with a body that tells it to re-authenticate
// and retry.
var ErrStepUpRequired = errors.New("step-up required")

// StepUpKey is where one person's marker lives.
func StepUpKey(personID int64) string {
	return fmt.Sprintf("%s%d", stepUpPrefix, personID)
}

// RequireStepUp reports whether a live proof exists for this person.
//
// A store failure is returned rather than swallowed, and callers must NOT
// treat it as "proved". This is the opposite of the rate limiter's fail-open:
// a limiter that cannot count should not lock everybody out of signing in,
// but a gate that cannot check should not let a credential-grade action
// through. Failing closed here costs somebody a retry; failing open costs
// somebody their profile.
func RequireStepUp(session KeyValueInterface, personID int64) error {
	if session == nil {
		return fmt.Errorf("step-up: no session store configured")
	}

	exists, err := session.Exists(StepUpKey(personID))
	if err != nil {
		return fmt.Errorf("step-up: read marker: %w", err)
	}

	if !exists {
		return ErrStepUpRequired
	}

	return nil
}

// ClearStepUp consumes the marker. One proof authorises one action.
//
// Best-effort by design: the action it authorised has already happened, and
// failing the request over an uncleared marker would report a success as a
// failure. The cost of a marker outliving its use is bounded by StepUpTTL.
func ClearStepUp(session KeyValueInterface, personID int64) error {
	if session == nil {
		return nil
	}
	return session.Del(StepUpKey(personID))
}
