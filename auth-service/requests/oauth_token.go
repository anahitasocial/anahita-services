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
	// `authorization_code`, `refresh_token` or `client_credentials`.
	GrantType string `form:"grant_type" validate:"required,oneof=authorization_code refresh_token client_credentials"`
	// The app's `client_id`, unless it is in an `Authorization: Basic` header.
	ClientID string `form:"client_id" validate:"omitempty"`
	// The app's secret, likewise; a public app has none.
	ClientSecret string `form:"client_secret" validate:"omitempty"`
	// For `authorization_code`: what the app was sent back with.
	Code string `form:"code" validate:"omitempty"`
	// For `authorization_code`: the one the code was asked for with.
	RedirectURI string `form:"redirect_uri" validate:"omitempty,url"`
	// For `authorization_code` with PKCE: the secret the challenge was the hash of.
	CodeVerifier string `form:"code_verifier" validate:"omitempty,min=43,max=128"`
	// For `refresh_token`.
	RefreshToken string `form:"refresh_token" validate:"omitempty"`
	// For `client_credentials`: what the app asks to do, separated by spaces.
	Scope string `form:"scope" validate:"omitempty"`
}

type OAuthRevokeReq struct {
	// The token.
	Token string `form:"token" validate:"required"`
	// `access_token` or `refresh_token`, to look for it as first.
	TokenTypeHint string `form:"token_type_hint" validate:"omitempty,oneof=access_token refresh_token"`
	// The app's `client_id`, unless it is in an `Authorization: Basic` header.
	ClientID string `form:"client_id" validate:"omitempty"`
	// The app's secret, likewise; a public app has none.
	ClientSecret string `form:"client_secret" validate:"omitempty"`
}

type OAuthIntrospectReq struct {
	// The token.
	Token string `form:"token" validate:"required"`
	// `access_token` or `refresh_token`, to look for it as first.
	TokenTypeHint string `form:"token_type_hint" validate:"omitempty,oneof=access_token refresh_token"`
	// The app's `client_id`, unless it is in an `Authorization: Basic` header.
	ClientID string `form:"client_id" validate:"omitempty"`
	// The app's secret, likewise; a public app has none.
	ClientSecret string `form:"client_secret" validate:"omitempty"`
}
