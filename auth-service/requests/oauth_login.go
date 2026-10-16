package requests

type OAuthLoginRequest struct {
	Login *OAuthLoginReq
}

func NewOAuthLoginRequest() *OAuthLoginRequest {
	return &OAuthLoginRequest{
		Login: &OAuthLoginReq{},
	}
}

// OAuthLoginCode is the first step of signing in with a code: who is asking.
// An email address or a username.
type OAuthLoginCode struct {
	// An email address or a username.
	Identifier string `form:"identifier"`
	// The token the page was given, which proves the form is this site's.
	CSRFToken string `form:"csrf_token"`
}

// OAuthLoginCodeMethod is where the code should go. Only "email" exists; an
// unknown value is read as email.
type OAuthLoginCodeMethod struct {
	// `email`.
	Channel string `form:"channel"`
	// The token the page was given, which proves the form is this site's.
	CSRFToken string `form:"csrf_token"`
}

// OAuthLoginCodeVerify is the six digits. Resend is set by the "send a new
// code" button on the same page.
type OAuthLoginCodeVerify struct {
	// The six digits that were sent.
	Code string `form:"code"`
	// The token the page was given, which proves the form is this site's.
	CSRFToken string `form:"csrf_token"`
}

// OAuthLoginReq is the form posted by the auth-service login page.
type OAuthLoginReq struct {
	// The email address.
	Identifier string `form:"identifier" validate:"required,email,max=128"`
	// The password.
	Password string `form:"password" validate:"required,min=8,max=72"`
	// The token the page was given, which proves it was this site's page.
	CSRFToken string `form:"csrf_token"`
}

// OAuthLoginTotp is the second step of signing in, for somebody with
// two-factor authentication: the passcode.
type OAuthLoginTotp struct {
	// The six digits the authenticator shows now.
	TotpPasscode string `form:"totp_passcode"`
	// The token the page was given, which proves the form is this site's.
	CSRFToken string `form:"csrf_token"`
}
