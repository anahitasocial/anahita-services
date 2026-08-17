package requests

type OAuthUserInfoRequest struct {
	Read *OAuthUserInfoReadReq
}

func NewOAuthUserInfoRequest() *OAuthUserInfoRequest {
	return &OAuthUserInfoRequest{
		Read: &OAuthUserInfoReadReq{},
	}
}

// OAuthUserInfoReadReq is intentionally body-less. The handler reads
// the subject from the validated JWT in context, not from request data.
// Kept for symmetry with other request structs.
type OAuthUserInfoReadReq struct {
	Subject string `validate:"omitempty"`
}
