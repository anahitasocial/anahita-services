package requests

// CodeForm is a six-digit code typed into one of the server-rendered pages.
type CodeForm struct {
	Code      string `form:"code"`
	CSRFToken string `form:"csrf_token"`
}

// ConfirmResend asks for a new confirmation code. Email is empty when it
// comes from the button on the page already waiting for one.
type ConfirmResend struct {
	Email     string `form:"email" validate:"required,email,max=255"`
	CSRFToken string `form:"csrf_token"`
}
