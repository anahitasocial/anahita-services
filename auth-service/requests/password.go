package requests

// PasswordForgot is submitted from the unauthenticated "forgot password"
// form. Only an email is required; the response is identical whether or
// not the email matches a real account (enumeration protection).
type PasswordForgot struct {
	Email string `form:"email" validate:"required,email"`
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
	Token    string `form:"token"    validate:"required,len=128"`
	Password string `form:"password" validate:"required,min=15,max=150"`
}

// PasswordEdit is submitted as JSON by the Anahita web app when a
// logged-in user changes their password via the settings page.
// Carries JSON tags rather than form tags because this is an API
// endpoint, not a server-rendered form.
//
// CurrentPassword is required as a re-authentication step — the IdP
// session alone is not sufficient proof for a credential mutation.
//
// No confirm field — the SPA form has a show/hide toggle on the new-
// password field, same reasoning as PasswordReset above.
type PasswordEdit struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	Password        string `json:"password"        validate:"required,min=15,max=150"`
}
