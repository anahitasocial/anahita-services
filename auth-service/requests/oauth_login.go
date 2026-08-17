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
type OAuthLoginReq struct {
	Identifier   string `form:"identifier" validate:"required,email,max=128"`
	Password     string `form:"password" validate:"required,min=8,max=72"`
	TotpPasscode string `form:"totp_passcode" validate:"omitempty,len=6"`
	ReturnTo     string `form:"return_to"`
}
