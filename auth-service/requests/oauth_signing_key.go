package requests

// OAuthSigningKeyReadReq backs GET /oauth-signing-keys/:kid.
//
// Kid is bound from the URL path via Echo's `param` tag. No body is
// expected; this struct exists so the handler can use the same
// Bind+Validate flow as every other handler instead of pulling
// c.Param manually.
type OAuthSigningKeyReadReq struct {
	// The key's `kid`.
	Kid string `param:"kid" validate:"required"`
}

// OAuthSigningKeyEditReq backs PATCH /oauth-signing-keys/:kid.
//
// Kid is bound from the URL path; Active from the JSON body. Echo's
// DefaultBinder runs path → query → body bindings in one Bind() call,
// so the handler gets both fields populated together.
//
// Active is a pointer to distinguish "not provided" (nil) from
// "explicitly false". The validator's `required` tag against the
// pointer ensures the client actually sent a value.
type OAuthSigningKeyEditReq struct {
	// The key's `kid`.
	Kid string `param:"kid" validate:"required"`
	// Whether to sign new tokens with it.
	Active *bool `json:"active" validate:"required"`
}

// OAuthSigningKeyRequests groups the request shapes for the
// oauth-signing-keys endpoints, following the OAuthTokenRequest
// convention. Handlers reach into the field for the endpoint they
// implement, e.g. `requests.NewOAuthSigningKeyRequest().Edit`.
type OAuthSigningKeyRequests struct {
	Read *OAuthSigningKeyReadReq
	Edit *OAuthSigningKeyEditReq
}

// NewOAuthSigningKeyRequest returns the grouped request bundle ready
// for c.Bind.
func NewOAuthSigningKeyRequest() *OAuthSigningKeyRequests {
	return &OAuthSigningKeyRequests{
		Read: &OAuthSigningKeyReadReq{},
		Edit: &OAuthSigningKeyEditReq{},
	}
}
