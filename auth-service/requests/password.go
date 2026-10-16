package requests

// PasswordForgot is submitted from the unauthenticated "forgot password"
// form. Only an email is required; the response is identical whether or
// not the email matches a real account (enumeration protection).
type PasswordForgot struct {
	// The account's address.
	Email string `form:"email" validate:"required,email"`
	// The token the page was given, which proves the form is this site's.
	CSRFToken string `form:"csrf_token"`
}

// PasswordReset is submitted from the new-password form rendered after
// the user clicks an email reset link. The token is carried in a hidden
// field on the form and re-validated server-side on submit.
//
// No confirm field — the form has a show/hide toggle on the password
// field so the user can verify what they typed, which makes the
// confirm field unnecessary friction (and actively harmful for
// password manager users who don't manually type either field).
type PasswordReset struct {
	// From the link, or the page after the code.
	Token string `form:"token"    validate:"required,len=128"`
	// The new password: at least 15 characters.
	Password string `form:"password" validate:"required,min=15,max=150"`
}

// PasswordEdit is submitted as JSON by anahita-web when a
// logged-in user changes their password via the settings page.
// Carries JSON tags rather than form tags because this is an API
// endpoint, not a server-rendered form.
//
// NO CurrentPassword FIELD. Re-authentication moved to /reauth/*, which
// records a marker this endpoint checks — the IdP session alone is
// still not sufficient proof, but the proof no longer travels here.
//
// That is what lets someone who signs in by passkey change their
// password without recalling the old one. Requiring it kept exactly
// those people out: their only route was forgot-password, and Reset
// deletes every passkey on the account.
//
// No confirm field — the SPA form has a show/hide toggle on the new-
// password field, same reasoning as PasswordReset above.
type PasswordEdit struct {
	// The new password: at least 15 characters.
	Password string `json:"password" validate:"required,min=15,max=150"`
}
