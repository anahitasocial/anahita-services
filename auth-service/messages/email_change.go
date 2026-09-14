package messages

const (
	// Email change confirmation — sent to the person's CURRENT address
	// when they ask to move to a new one.
	//
	// Goes to the old address on purpose. The current inbox is the thing
	// an attacker holding a live session does not have, so requiring it
	// to approve the move is what stops a session hijack from becoming a
	// permanent account takeover.
	//
	// The new address is named in full in the body so the person can see
	// what they are approving. "Confirm a change" without saying to what
	// is not consent to anything.
	EmailChangeSubject     = "Confirm your new email address"
	EmailChangeTitle       = "Confirm your email change"
	EmailChangeBody        = "Hello %s, we received a request to change the email address on your {{site}} account to %s. Click the button below to confirm — you will be asked to re-type the new address. This link expires in 15 minutes. If you did not request this, do not click the button: your address has not been changed, and you should change your password immediately, then contact support."
	EmailChangeActionLabel = "Confirm email change"

	// Sent to the OLD address after the change completes. Recovery here
	// is deliberately not a link — by this point the person no longer
	// controls the account's address, so a self-service reset would not
	// reach them and pointing them at one would waste the minutes that
	// matter. Support is the only real path back.
	EmailChangedOldSubject     = "Your {{site}} email address was changed"
	EmailChangedOldTitle       = "Email address changed"
	EmailChangedOldBody        = "Hello %s, the email address on your {{site}} account was changed from %s to %s. This address will no longer receive sign-in or account notifications. If you did not do this, contact support immediately — password reset will no longer reach you, so this is the only way to recover the account."
	EmailChangedOldActionLabel = "Contact support"

	// Sent to the NEW address after the change completes, so the person
	// has confirmation it works and something to find later.
	// Passkeys are named because the change removes every one of them
	// (email_change.go calls DeleteByPersonID), and until this said so the
	// person simply found them gone. Somebody who signs in by passkey daily
	// would reach for one, fail, and have nothing connecting that to the
	// address change they made a minute earlier.
	EmailChangedNewSubject     = "Your {{site}} email address was updated"
	EmailChangedNewTitle       = "Email address updated"
	EmailChangedNewBody        = "Hello %s, this address is now the email address on your {{site}} account. You have been signed out everywhere as a precaution, and any passkeys were removed — sign in again with this address and your password, then add your passkeys back from your security settings. If you did not do this, contact support immediately."
	EmailChangedNewActionLabel = "Sign in"
)
