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
)
