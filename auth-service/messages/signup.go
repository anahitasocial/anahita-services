package messages

// The welcome message is where naming the site matters most for a plain
// reason: it is the first mail somebody gets, and "Welcome" alone does not say
// what they have joined.
const (
	SignupSubject     = "Welcome to {{site}}"
	SignupTitle       = "Welcome to {{site}}"
	SignupBody        = "Hi %s, thanks for signing up to {{site}}. Your account is ready to use."
	SignupActionLabel = "Get started"

	SignupConfirmSubject     = "Confirm your email address"
	SignupConfirmTitle       = "Confirm your email address"
	SignupConfirmBody        = "Hi %s, please confirm this is your email address so you can sign in to {{site}}. This link expires in 24 hours."
	SignupConfirmActionLabel = "Confirm my email"

	SignupRequestReceivedSubject     = "Someone has asked to join {{site}}"
	SignupRequestReceivedTitle       = "A new signup request"
	SignupRequestReceivedBody        = "Hi %s, %s (@%s) has asked to join {{site}} and is waiting for a decision.\n\nThey said:\n%s"
	SignupRequestReceivedActionLabel = "Review signup requests"

	SignupRequestApprovedSubject     = "You're in"
	SignupRequestApprovedTitle       = "Welcome to {{site}}"
	SignupRequestApprovedBody        = "Hi %s, your request to join {{site}} was approved. You can sign in with the username and password you chose."
	SignupRequestApprovedActionLabel = "Sign in"

	SignupRequestRejectedSubject     = "About your request to join {{site}}"
	SignupRequestRejectedTitle       = "Your request to join"
	SignupRequestRejectedBody        = "Hi %s, thank you for your interest in {{site}}. Your request to join was not accepted this time."
	SignupRequestRejectedActionLabel = "Visit {{site}}"
)
