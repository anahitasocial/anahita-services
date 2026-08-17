package responses

import "auth/data/entities"

type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope"`
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
