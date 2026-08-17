package entities

// OAuthTokenPair is the response body for a successful /token exchange.
// Matches RFC 6749 §5.1 plus OIDC Core for id_token.
//
// Lives in the entities package alongside other OAuth entities so that
// response mappers can import a single namespace.
type OAuthTokenPair struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope"`
}

// OAuthTokenInfo is the response body for a successful /introspect call.
// Matches RFC 7662 §2.2 for standard fields, plus the Anahita
// org_id custom claim.
type OAuthTokenInfo struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Sub       string `json:"sub,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
	Iat       int64  `json:"iat,omitempty"`
	Iss       string `json:"iss,omitempty"`
	Jti       string `json:"jti,omitempty"`
	TokenType string `json:"token_type,omitempty"`
}
