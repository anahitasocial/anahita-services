package requests

type OAuthConsentRequest struct {
	Show *OAuthConsentShowReq
}

func NewOAuthConsentRequest() *OAuthConsentRequest {
	return &OAuthConsentRequest{
		Show: &OAuthConsentShowReq{},
	}
}

type OAuthConsentShowReq struct {
	SessionID string `query:"session_id" validate:"required"`
}
