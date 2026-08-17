package responses

type OAuthUserInfoResponse struct{}

func (r *OAuthUserInfoResponse) MapUserInfo(info map[string]any) map[string]any {
	return info
}
