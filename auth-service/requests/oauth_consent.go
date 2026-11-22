package requests

type OAuthConsentRequest struct {
	Show *OAuthConsentShowReq
}

type OAuthConsentShowReq struct {
	SessionID string `query:"session_id" validate:"required"`
}
