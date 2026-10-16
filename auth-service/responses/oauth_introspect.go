package responses

import "auth/data/entities"

type OAuthIntrospectItem struct {
	// Whether it is live. Nothing else is said of one that is not.
	Active bool `json:"active"`
	// Whose it is: the person's id, or the app's for its own.
	Sub string `json:"sub,omitempty"`
	// The app it was issued to.
	ClientID string `json:"client_id,omitempty"`
	// What it may do, separated by spaces.
	Scope string `json:"scope,omitempty"`
	// When it expires, in seconds since 1970.
	Exp int64 `json:"exp,omitempty"`
	// When it was issued, likewise.
	Iat int64 `json:"iat,omitempty"`
	// Who issued it: this installation.
	Iss string `json:"iss,omitempty"`
	// Its own id.
	Jti string `json:"jti,omitempty"`
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
