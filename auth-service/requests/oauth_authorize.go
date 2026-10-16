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
	// `code`.
	ResponseType string `query:"response_type" validate:"required,oneof=code"`
	// The app's `client_id`.
	ClientID string `query:"client_id" validate:"required"`

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

	// What the app asks to do, separated by spaces (oauth.md).
	Scope string `query:"scope" validate:"required"`
	// Anything; it comes back unchanged with the code, for the app to check.
	State string `query:"state" validate:"required"`
	// PKCE (RFC 7636): the hash of a secret only the app knows, given again with the code.
	CodeChallenge string `query:"code_challenge" validate:"omitempty,min=43,max=128"`
	// `S256` or `plain`.
	CodeChallengeMethod string `query:"code_challenge_method" validate:"omitempty,oneof=S256 plain"`
	// Anything; it is put in the ID token (OpenID Connect).
	Nonce string `query:"nonce" validate:"omitempty"`
}

type OAuthAuthorizeCallbackReq struct {
	// Whether the person allows it.
	Approved bool `form:"approved"`
	// The token the page was given, which proves the form is this site's.
	CSRFToken string `form:"csrf_token" validate:"required"`
}
