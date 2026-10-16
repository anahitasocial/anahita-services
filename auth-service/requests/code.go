package requests

// CodeForm is a six-digit code typed into one of the server-rendered pages.
type CodeForm struct {
	// The six digits that were mailed.
	Code string `form:"code"`
	// The token the page was given, which proves the form is this site's.
	CSRFToken string `form:"csrf_token"`
}

// ConfirmResend asks for a new confirmation code. Email is empty when it
// comes from the button on the page already waiting for one.
type ConfirmResend struct {
	// The address signed up with.
	Email string `form:"email" validate:"required,email,max=255"`
	// The token the page was given, which proves the form is this site's.
	CSRFToken string `form:"csrf_token"`
}
