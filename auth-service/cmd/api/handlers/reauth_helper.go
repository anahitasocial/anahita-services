package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ---------------------------------------------------------------------
// Step-up markers
// ---------------------------------------------------------------------
//
// A step-up is a short-lived server-side note that says "this person
// proved who they are, just now, with something stronger than the
// session cookie". The /reauth/* endpoints write it; endpoints that
// mutate credentials read it.
//
// Splitting it out from the endpoints that check it is what lets the
// proof and the mutation be different requests. That in turn is what
// lets a passkey stand in for a password: a WebAuthn assertion cannot
// travel in a JSON body next to the new password, but it can be the
// thing that wrote this marker a minute ago.
//
// Keyed by person, not by session. The BFF forwards these calls with
// an X-OAuth-Token rather than the auth host's cookie, so there is no
// single session identifier reliably present on both the /reauth/*
// request and the request that follows it. Person is what both have in
// common. The practical consequence is that a step-up performed in one
// browser authorises a change made in another within the window; the
// bound on that is stepUpTTL, and it is short for this reason.

// stepUpTTL bounds how long a proof stays good for.
//
// Five minutes: long enough to reach for a phone and read a passcode,
// short enough that an unlocked laptop left alone is not a standing
// authorisation to change the password. Deliberately longer than
// pendingTOTPTTL, which bounds a single ceremony rather than a window
// the person navigates a UI in.
const stepUpTTL = 5 * time.Minute

const stepUpPrefix = "stepup:"

// errStepUpRequired means no live proof exists for this person. Not an
// error condition in the usual sense — it is the expected answer the
// first time a client hits a guarded endpoint, and the client's job is
// to re-authenticate and retry.
var errStepUpRequired = errors.New("step-up required")

func stepUpKey(personID int64) string {
	return fmt.Sprintf("%s%d", stepUpPrefix, personID)
}

// markStepUp records a successful re-authentication.
//
// Called by the /reauth/* endpoints after — and only after — a proof
// verifies. The Del before the write is what refreshes the expiry: the
// store's set-if-absent leaves an existing key's TTL untouched, so
// without it a second step-up inside the window would verify a proof
// and grant nothing extra for it.
//
// Errors are returned rather than swallowed. A marker that silently
// failed to write leaves the person re-authenticating in a loop with
// no explanation, which reads as the credential change being broken.
func (env *Env) markStepUp(personID int64) error {
	key := stepUpKey(personID)

	if err := env.Session.Del(key); err != nil {
		return fmt.Errorf("clear previous step-up: %w", err)
	}

	if _, err := env.Session.SetNX(key, "1", stepUpTTL); err != nil {
		return fmt.Errorf("write step-up: %w", err)
	}

	return nil
}

// requireStepUp reports whether a live proof exists for this person.
//
// Returns errStepUpRequired when there is none, and a wrapped store
// error when the question could not be answered.
//
// Fails CLOSED, unlike rateLimitExceeded. The two are not inconsistent:
// the limiter degrading open costs the exposure the service had before
// it existed, whereas this degrading open would hand every credential
// mutation to anyone holding a session — the exact thing it was added
// to stop. A store outage should make password changes unavailable,
// not unguarded.
func (env *Env) requireStepUp(personID int64) error {
	exists, err := env.Session.Exists(stepUpKey(personID))
	if err != nil {
		return fmt.Errorf("read step-up marker: %w", err)
	}

	if !exists {
		return errStepUpRequired
	}

	return nil
}

// clearStepUp consumes the marker.
//
// One step-up authorises one credential change. Called after a change
// succeeds, so a person changing their password and then their email
// proves themselves twice — which is the point, since each of those is
// separately worth stealing.
//
// Best-effort: the change has already happened, and failing the request
// over an uncleared marker would report a success as a failure. The
// cost of a marker that outlives its use is bounded by stepUpTTL.
func (env *Env) clearStepUp(personID int64) {
	if err := env.Session.Del(stepUpKey(personID)); err != nil {
		log.Printf("step-up: clear failed (person=%d): %v", personID, err)
	}
}

// reauthStatus maps a requireStepUp error to a status code.
//
// 403 is a routing instruction to the client, not a refusal: the SPA
// catches it, opens the step-up dialog, and retries what it was doing.
// A store failure must NOT come back as 403 — the client would prompt
// for a passkey, write nothing, retry, and get 403 again, blaming the
// person for an outage. 500 says the truth, which is that this is our
// problem and retrying now will not help.
func reauthStatus(err error) int {
	if errors.Is(err, errStepUpRequired) {
		return http.StatusForbidden
	}
	return http.StatusInternalServerError
}
