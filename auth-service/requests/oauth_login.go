package requests

type OAuthLoginRequest struct {
	Login *OAuthLoginReq
}

func NewOAuthLoginRequest() *OAuthLoginRequest {
	return &OAuthLoginRequest{
		Login: &OAuthLoginReq{},
	}
}

// OAuthLoginReq is the form posted by the auth-service login page.
// Identifier is always an email in anahita (no usernames).
// TotpPasscode is conditionally required by the handler if the person
// has TOTP enabled.
// OAuthLoginCode is the first step of signing in with a code: who is asking.
// An email address or a username.
type OAuthLoginCode struct {
	Identifier string `form:"identifier"`
	CSRFToken  string `form:"csrf_token"`
}

// OAuthLoginCodeMethod is where the code should go. Only "email" exists; an
// unknown value is read as email.
type OAuthLoginCodeMethod struct {
	Channel   string `form:"channel"`
	CSRFToken string `form:"csrf_token"`
}

// OAuthLoginCodeVerify is the six digits. Resend is set by the "send a new
// code" button on the same page.
type OAuthLoginCodeVerify struct {
	Code      string `form:"code"`
	CSRFToken string `form:"csrf_token"`
}

type OAuthLoginReq struct {
	Identifier   string `form:"identifier" validate:"required,email,max=128"`
	Password     string `form:"password" validate:"required,min=8,max=72"`
	TotpPasscode string `form:"totp_passcode" validate:"omitempty,len=6"`
	ReturnTo     string `form:"return_to"`
	CSRFToken    string `form:"csrf_token"`
}
