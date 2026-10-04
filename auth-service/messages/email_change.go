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
	// The code is not in the body: the template shows it by itself, boxed,
	// between the title and this text. The new address IS in the body: it is
	// what the owner is approving.
	//
	// Verbs: the code (subject); name, the new address (body).
	EmailChangeCodeSubject = "%s is your {{site}} email change code"
	EmailChangeCodeTitle   = "Confirm your email change"
	EmailChangeCodeBody    = "Hello %s, we received a request to change the email address on your {{site}} account to %s. To approve it, enter this code on the settings page where you asked for the change. It expires in 15 minutes. We will never ask you for this code: do not share it with anyone. If you did not request this, do not enter the code, and change your password: somebody is signed in to your account."

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
