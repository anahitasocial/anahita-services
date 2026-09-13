package messages

const (
	// Sent after a username change completes.
	//
	// A notification rather than a confirmation: unlike an email change,
	// this one applies immediately. The distinction is that a username
	// change moves no recovery path — password reset still delivers to
	// the same inbox — so there is nothing here that requires the
	// mailbox's approval before it happens. What the mailbox is for is
	// noticing.
	//
	// Both handles are named. "Your username was changed" without saying
	// from what and to what tells somebody being impersonated nothing
	// they can act on.
	//
	// Recovery is a password change, not a reset: an unexpected message
	// here means somebody reached a step-up gate, which takes a passkey
	// or the password — so the password is what to assume is known.
	UsernameChangedSubject     = "Your {{site}} username was changed"
	UsernameChangedTitle       = "Username changed"
	UsernameChangedBody        = "Hello %s, the username on your {{site}} account was changed from %s to %s. You can sign in with either the new username or your email address. Links to your old profile address will no longer reach you. If you did not do this, change your password immediately and then contact support."
	UsernameChangedActionLabel = "Change password"
)
