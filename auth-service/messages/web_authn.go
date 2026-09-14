package messages

const (
	// Passkey added — neutral. Enrolling a passkey makes the account
	// harder to attack, and alarming people about an improvement is
	// how you get them to stop reading security email. The recovery
	// path is still spelled out for the case where it wasn't them.
	//
	// Three verbs: first name, last name, passkey nickname. The
	// nickname is derived server-side from the authenticator, so it
	// reads like "iCloud Keychain" or "Windows Hello" — the same
	// words the person saw in their browser's prompt.
	WebAuthnAddedSubject     = "A passkey was added to your {{site}} account"
	WebAuthnAddedTitle       = "Passkey added"
	WebAuthnAddedBody        = "Hello %s, a passkey named \"%s\" was just added to your {{site}} account. You can now sign in with your fingerprint, face, or device PIN instead of your password. If this wasn't you, reset your password immediately and contact support."
	WebAuthnAddedActionLabel = "Reset password"

	// Passkey removed — firmer. Removing a sign-in method is the
	// direction an account takeover travels in. Not framed as a
	// lockout, because it isn't one: password and TOTP remain, and
	// telling someone they've lost access when they haven't sends
	// them to support for no reason.
	//
	// Three verbs: first name, last name, passkey nickname.
	WebAuthnRemovedSubject     = "A passkey was removed from your {{site}} account"
	WebAuthnRemovedTitle       = "Passkey removed"
	WebAuthnRemovedBody        = "Hello %s, the passkey named \"%s\" was just removed from your {{site}} account. You can still sign in with your email, password, and verification code. If this wasn't you, reset your password immediately and contact support."
	WebAuthnRemovedActionLabel = "Reset password"

	// Passkeys revoked by an administrator. Firmer than the
	// single-removal notice, because the person did not do this and
	// the first thing they will wonder is whether they still have an
	// account.
	//
	// Two bodies rather than one with "passkey(s)". Go has no plural
	// machinery, and this is a security email — the kind people
	// actually read, and the kind where a sloppy "1 passkey(s)" makes
	// the whole thing look automated and safe to ignore. Both take
	// the same three verbs (first name, last name, count), the
	// singular accepting and discarding the count, so the two are
	// interchangeable at the call site.
	WebAuthnRevokedAllSubject     = "Your {{site}} passkeys were removed"
	WebAuthnRevokedAllTitle       = "Passkeys removed"
	WebAuthnRevokedAllBodyOne     = "Hello %s, a super administrator removed the %d passkey on your {{site}} account. You can still sign in with your email, password, and verification code, and you can add new passkeys at any time. If you weren't expecting this, contact support."
	WebAuthnRevokedAllBody        = "Hello %s, a super administrator removed all %d passkeys from your {{site}} account. You can still sign in with your email, password, and verification code, and you can add new passkeys at any time. If you weren't expecting this, contact support."
	WebAuthnRevokedAllActionLabel = "Contact support"
)
