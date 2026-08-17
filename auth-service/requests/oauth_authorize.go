package requests

type OAuthAuthorizeRequest struct {
	Authorize *OAuthAuthorizeReq
	Callback  *OAuthAuthorizeCallbackReq
}

func NewOAuthAuthorizeRequest() *OAuthAuthorizeRequest {
	return &OAuthAuthorizeRequest{
		Authorize: &OAuthAuthorizeReq{},
		Callback:  &OAuthAuthorizeCallbackReq{},
	}
}

type OAuthAuthorizeReq struct {
	ResponseType string `query:"response_type" validate:"required,oneof=code"`
	ClientID     string `query:"client_id" validate:"required"`

	// RedirectURI uses the `uri` tag, not `url`. The `url` tag in
	// go-playground/validator rejects any URI whose scheme isn't
	// http/https — but RFC 8252 (OAuth 2.0 for Native Apps)
	// explicitly recommends custom schemes for desktop and mobile
	// apps because they
	// route the callback to the registered app via the OS rather
	// than through a browser. The `uri` tag accepts any valid URI
	// with a scheme, which is the right syntactic check here.
	//
	// The real security check — that the URI matches one of the
	// client's registered redirect URIs exactly — happens later in
	// the handler against the client record. This validation is
	// just a syntactic sanity check; the exact-match comparison is
	// the actual gate.
	RedirectURI string `query:"redirect_uri" validate:"required,uri"`

	Scope               string `query:"scope" validate:"required"`
	State               string `query:"state" validate:"required"`
	CodeChallenge       string `query:"code_challenge" validate:"omitempty,min=43,max=128"`
	CodeChallengeMethod string `query:"code_challenge_method" validate:"omitempty,oneof=S256 plain"`
	Nonce               string `query:"nonce" validate:"omitempty"`
}

type OAuthAuthorizeCallbackReq struct {
	Approved  bool   `form:"approved"`
	CSRFToken string `form:"csrf_token" validate:"required"`
}
