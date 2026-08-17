package responses

import "auth/data/entities"

type OAuthIntrospectItem struct {
	Active   bool   `json:"active"`
	Sub      string `json:"sub,omitempty"`
	ClientID string `json:"client_id,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Exp      int64  `json:"exp,omitempty"`
	Iat      int64  `json:"iat,omitempty"`
	Iss      string `json:"iss,omitempty"`
	Jti      string `json:"jti,omitempty"`
}

type OAuthIntrospectResponse struct{}

func (r *OAuthIntrospectResponse) MapTokenInfo(info *entities.OAuthTokenInfo) *OAuthIntrospectItem {
	if info == nil {
		return &OAuthIntrospectItem{Active: false}
	}

	return &OAuthIntrospectItem{
		Active:   info.Active,
		Sub:      info.Sub,
		ClientID: info.ClientID,
		Scope:    info.Scope,
		Exp:      info.Exp,
		Iat:      info.Iat,
		Iss:      info.Iss,
		Jti:      info.Jti,
	}
}

func (r *OAuthIntrospectResponse) MapInactive() *OAuthIntrospectItem {
	return &OAuthIntrospectItem{Active: false}
}
