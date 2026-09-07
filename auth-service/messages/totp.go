package messages

const (
	// TOTP enabled — neutral tone. Enabling 2FA strengthens the
	// account; an alarmist alert here trains users to ignore future
	// security emails.
	TotpEnabledSubject     = "Two-factor authentication enabled on {{site}}"
	TotpEnabledTitle       = "Two-factor authentication enabled"
	TotpEnabledBody        = "Hello %s %s, two-factor authentication was just enabled on your {{site}} account. Your account is now more secure. If this wasn't you, contact support."
	TotpEnabledActionLabel = "Contact support"

	// TOTP disabled — urgent. This is the dangerous direction; an
	// attacker disabling 2FA is preparing to take over.
	TotpDisabledSubject     = "Two-factor authentication disabled on {{site}}"
	TotpDisabledTitle       = "Two-factor authentication disabled"
	TotpDisabledBody        = "Hello %s %s, two-factor authentication was just disabled on your {{site}} account. If this was you, no further action is needed. If it wasn't, reset your password immediately and contact support."
	TotpDisabledActionLabel = "Reset password"

	// Disabled by an ADMINISTRATOR rather than by the owner.
	//
	// A separate body because the one above asks "was this you?", and for
	// somebody who did not touch their account that question is confusing —
	// they will answer no and reset a password that was never at risk. Naming
	// the actor is also what makes the message actionable: the recovery here
	// is to ask why, not to assume compromise.
	//
	// Recovery is support rather than a password reset, on the same reasoning
	// as the admin passkey revocation below it: an administrator acting on an
	// account is not a credential problem.
	//
	// Two verbs: given name, family name.
	TotpDisabledByAdminSubject     = "An administrator turned off two-factor authentication"
	TotpDisabledByAdminTitle       = "Two-factor authentication disabled"
	TotpDisabledByAdminBody        = "Hello %s %s, an administrator has turned off two-factor authentication on your {{site}} account. You can sign in with your password alone until you enable it again, so turning it back on from your security settings is worth doing soon. If you were not expecting this, contact support."
	TotpDisabledByAdminActionLabel = "Contact support"

	// Recovery codes issued or reissued.
	//
	// Worth a message because generating a set silently invalidates the
	// previous one and mints a fresh way past 2FA. Somebody who did not do
	// this is looking at an attacker who holds both their password and a live
	// passcode, and who has just given themselves a durable bypass.
	//
	// Neutral-to-firm: reissuing your own codes is routine, so the alarm is
	// held back for the "if this wasn't you" branch.
	//
	// Three verbs: given name, family name, number of codes issued.
	TotpRecoveryCodesIssuedSubject     = "New two-factor recovery codes were generated"
	TotpRecoveryCodesIssuedTitle       = "New recovery codes generated"
	TotpRecoveryCodesIssuedBody        = "Hello %s %s, %d new two-factor recovery codes were just generated for your {{site}} account. Any codes you were given before this no longer work. If you did not do this, somebody else can now get past two-factor authentication on your account: change your password immediately and then contact support."
	TotpRecoveryCodesIssuedActionLabel = "Change password"

	// A recovery code was used to sign in.
	//
	// This is the notification with the least competition: a recovery code is
	// a single-use bypass of the second factor, and using one leaves no other
	// trace the owner would ever look at. If somebody else used it, this
	// message is the only way they find out.
	//
	// The remaining count is included because it is genuinely useful in both
	// readings — routine ("two left, time to reissue") and hostile ("I have
	// used none of these").
	//
	// Three verbs: given name, family name, number of codes remaining.
	TotpRecoveryCodeUsedSubject     = "A recovery code was used to sign in"
	TotpRecoveryCodeUsedTitle       = "Recovery code used"
	TotpRecoveryCodeUsedBody        = "Hello %s %s, somebody signed in to your {{site}} account using a two-factor recovery code instead of a passcode. That code has now been used up and %d remain. If this was not you, change your password immediately and then contact support."
	TotpRecoveryCodeUsedActionLabel = "Change password"

	// Singular and the empty case. Two more constants rather than
	// "%d remain(s)" — this is a security email, and a sentence that reads
	// like a debug line is one people learn to skim.
	//
	// Two verbs on each: given name, family name. The count is in the wording
	// rather than a verb, which is the point of having separate constants.
	TotpRecoveryCodeUsedBodyOne  = "Hello %s %s, somebody signed in to your {{site}} account using a two-factor recovery code instead of a passcode. That code has now been used up and one remains. If this was not you, change your password immediately and then contact support."
	TotpRecoveryCodeUsedBodyNone = "Hello %s %s, somebody signed in to your {{site}} account using a two-factor recovery code instead of a passcode. That was your last one — generate a new set from your security settings, or you may be locked out if you lose your authenticator. If this was not you, change your password immediately and then contact support."

	// Too many failed passcode attempts.
	//
	// Somebody is guessing at a live account, and until now this reached
	// stdout and nowhere else. Firm, because a lockout means an attacker
	// already has the password — the second factor is the only thing that
	// stopped them.
	//
	// Two verbs: given name, family name.
	TotpLockedOutSubject     = "Too many two-factor attempts on your account"
	TotpLockedOutTitle       = "Two-factor sign-in blocked"
	TotpLockedOutBody        = "Hello %s %s, somebody entered the wrong two-factor passcode too many times on your {{site}} account, and that sign-in attempt was blocked. Reaching this step needs your password, so if it was not you, treat your password as known to somebody else: change it immediately and then contact support."
	TotpLockedOutActionLabel = "Change password"
)
