package messages

// The welcome message is where naming the site matters most for a plain
// reason: it is the first mail somebody gets, and "Welcome" alone does not say
// what they have joined.
const (
	SignupSubject     = "Welcome to {{site}}"
	SignupTitle       = "Welcome to {{site}}"
	SignupBody        = "Hi %s, thanks for signing up to {{site}}. Your account is ready to use."
	SignupActionLabel = "Get started"

	// The confirmation code. The code is not in the body: the template shows
	// it by itself, boxed, between the title and this text.
	//
	// Verbs: the code (subject); handle (body).
	SignupConfirmCodeSubject = "%s is your {{site}} confirmation code"
	SignupConfirmCodeTitle   = "Confirm your email address"
	SignupConfirmCodeBody    = "Hi %s, enter this code on the page you signed up on to confirm this is your email address. It expires in 15 minutes. We will never ask you for this code: do not share it with anyone. If you did not sign up to {{site}}, you can ignore this message."

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
