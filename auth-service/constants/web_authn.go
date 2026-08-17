package constants

import "time"

const (
	// WebAuthnCeremonyTTL bounds how long a pending challenge stays
	// valid. Matches pendingTOTPTTL for consistency: long enough to
	// find your phone, short enough that a stolen mid-ceremony session
	// is not a standing invitation.
	//
	// The browser also enforces its own timeout on the prompt; this is
	// the server-side backstop, and it is the one that matters,
	// because the browser's can be ignored by anything that isn't a
	// browser.
	WebAuthnCeremonyTTL = 2 * time.Minute

	// WebAuthnMaxCredentialsPerPerson caps enrolments. Not a security
	// boundary — a person with ten passkeys is no more dangerous than
	// a person with one — but an unbounded list is a list nobody
	// audits, and the exclusion list sent to the browser at
	// registration grows with it.
	WebAuthnMaxCredentialsPerPerson = 10

	// WebAuthnNicknameMaxLength bounds the label. Applies to renames
	// only — the name assigned at registration is derived from a
	// fixed table and is always well within this.
	WebAuthnNicknameMaxLength = 64
)
