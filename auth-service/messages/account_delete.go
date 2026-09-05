package messages

const (
	// Sent when an account is scheduled for deletion.
	//
	// This is the tripwire as much as it is a receipt: if the person did not do
	// this, somebody else reached a step-up on their account, and the grace
	// period is the window in which that can still be undone. So the message
	// leads with the date and with how to stop it, not with a farewell.
	AccountDeleteScheduledSubject     = "Your {{site}} account is scheduled for deletion"
	AccountDeleteScheduledTitle       = "Account scheduled for deletion"
	AccountDeleteScheduledBody        = "Hello %s %s, your {{site}} account has been scheduled for deletion and will be permanently erased on %s. Until then nothing has been lost: signing in with your password will offer to restore it. If you did not request this, sign in now to restore your account and then change your password."
	AccountDeleteScheduledActionLabel = "Restore my account"

	// Sent after a restore.
	//
	// Also a tripwire, in the other direction: an unexpected restore means
	// somebody knows the password of an account that was on its way out.
	AccountRestoredSubject     = "Your {{site}} account has been restored"
	AccountRestoredTitle       = "Account restored"
	AccountRestoredBody        = "Hello %s %s, your {{site}} account has been restored and is no longer scheduled for deletion. Sessions and passkeys were revoked when the deletion was requested, so you will need to sign in again on your other devices and add any passkeys back. If you did not do this, change your password immediately and then contact support."
	AccountRestoredActionLabel = "Sign in"
)
