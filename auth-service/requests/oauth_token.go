package requests

type OAuthTokenRequest struct {
	Token      *OAuthTokenReq
	Revoke     *OAuthRevokeReq
	Introspect *OAuthIntrospectReq
}

func NewOAuthTokenRequest() *OAuthTokenRequest {
	return &OAuthTokenRequest{
		Token:      &OAuthTokenReq{},
		Revoke:     &OAuthRevokeReq{},
		Introspect: &OAuthIntrospectReq{},
	}
}

type OAuthTokenReq struct {
	GrantType    string `form:"grant_type" validate:"required,oneof=authorization_code refresh_token client_credentials"`
	ClientID     string `form:"client_id" validate:"omitempty"`
	ClientSecret string `form:"client_secret" validate:"omitempty"`
	Code         string `form:"code" validate:"omitempty"`
	RedirectURI  string `form:"redirect_uri" validate:"omitempty,url"`
	CodeVerifier string `form:"code_verifier" validate:"omitempty,min=43,max=128"`
	RefreshToken string `form:"refresh_token" validate:"omitempty"`
	Scope        string `form:"scope" validate:"omitempty"`
}

type OAuthRevokeReq struct {
	Token         string `form:"token" validate:"required"`
	TokenTypeHint string `form:"token_type_hint" validate:"omitempty,oneof=access_token refresh_token"`
	ClientID      string `form:"client_id" validate:"omitempty"`
	ClientSecret  string `form:"client_secret" validate:"omitempty"`
}

type OAuthIntrospectReq struct {
	Token         string `form:"token" validate:"required"`
	TokenTypeHint string `form:"token_type_hint" validate:"omitempty,oneof=access_token refresh_token"`
	ClientID      string `form:"client_id" validate:"omitempty"`
	ClientSecret  string `form:"client_secret" validate:"omitempty"`
}
