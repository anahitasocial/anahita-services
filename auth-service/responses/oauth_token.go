package responses

import "auth/data/entities"

type OAuthTokenResponse struct {
	// What a request is signed with: `Authorization: Bearer <token>`. A JWT.
	AccessToken string `json:"access_token"`
	// `Bearer`.
	TokenType string `json:"token_type"`
	// How long the access token lasts, in seconds.
	ExpiresIn int `json:"expires_in"`
	// What gets the next tokens when this one expires; used once. Not for an app acting for itself.
	RefreshToken string `json:"refresh_token,omitempty"`
	// Who signed in, when `openid` was asked for (OpenID Connect).
	IDToken string `json:"id_token,omitempty"`
	// What the tokens may do, separated by spaces.
	Scope string `json:"scope"`
}

// MapTokenPair builds the response body for authorization_code and
// refresh_token grants — includes refresh_token and (if present) id_token.
func (r *OAuthTokenResponse) MapTokenPair(tp *entities.OAuthTokenPair) *OAuthTokenResponse {
	return &OAuthTokenResponse{
		AccessToken:  tp.AccessToken,
		TokenType:    tp.TokenType,
		ExpiresIn:    tp.ExpiresIn,
		RefreshToken: tp.RefreshToken,
		IDToken:      tp.IDToken,
		Scope:        tp.Scope,
	}
}

// MapAccessToken builds the response body for the client_credentials
// grant — no refresh_token, no id_token.
func (r *OAuthTokenResponse) MapAccessToken(tp *entities.OAuthTokenPair) *OAuthTokenResponse {
	return &OAuthTokenResponse{
		AccessToken: tp.AccessToken,
		TokenType:   tp.TokenType,
		ExpiresIn:   tp.ExpiresIn,
		Scope:       tp.Scope,
	}
}
