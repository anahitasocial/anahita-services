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

	ApplicationReceivedSubject     = "Someone has asked to join {{site}}"
	ApplicationReceivedTitle       = "A new application"
	ApplicationReceivedBody        = "Hi %s, %s (@%s) has asked to join {{site}} and is waiting for a decision.\n\nThey said:\n%s"
	ApplicationReceivedActionLabel = "Review applications"

	ApplicationApprovedSubject     = "You're in"
	ApplicationApprovedTitle       = "Welcome to {{site}}"
	ApplicationApprovedBody        = "Hi %s, your application to join {{site}} was approved. You can sign in with the username and password you chose."
	ApplicationApprovedActionLabel = "Sign in"

	ApplicationRejectedSubject     = "About your application to {{site}}"
	ApplicationRejectedTitle       = "Your application"
	ApplicationRejectedBody        = "Hi %s, thank you for your interest in {{site}}. Your application was not accepted this time."
	ApplicationRejectedActionLabel = "Visit {{site}}"
)
