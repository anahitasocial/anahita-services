package requests

// EmailChangeEdit starts an email change. Submitted as JSON by
// anahita-web from the settings page.
//
// CARRIES NO PROOF OF IDENTITY, deliberately. Re-authentication happens
// separately against /reauth/*, which records a short-lived marker this
// endpoint checks. Threading the proof through here would mean
// duplicating the per-account branching (passkey alone, or password
// plus passcode, or password) across three endpoints, and a passkey
// ceremony is two round trips that cannot be folded into this body at
// all.
type EmailChangeEdit struct {
	Email string `json:"email" validate:"required,email,min=10,max=100"`
}

// EmailChangeConfirm is submitted from the server-rendered confirmation
// page reached from the link mailed to the person's CURRENT address.
//
// Email is the new address, re-typed by the person rather than carried
// in a hidden field. That is the typo guard: confirming at the old
// address is what defeats a stolen session, but it also means a
// mistyped destination would move the account to an inbox the person
// cannot reach. Asking them to type it while looking at it costs one
// field and removes the lockout.
type EmailChangeConfirm struct {
	Code string `json:"code"`
}
