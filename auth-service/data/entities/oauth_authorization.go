package entities

import (
	"errors"
	"strings"
	"time"
)

// PKCE code challenge methods (RFC 7636).
const (
	CodeChallengeMethodPlain = "plain"
	CodeChallengeMethodS256  = "S256"
)

// OAuthAuthorization represents a short-lived authorization code issued at
// /authorize, to be exchanged at /token for an access + refresh token pair.
// Codes are single-use and typically expire in ~10 minutes.
type OAuthAuthorization struct {
	ID int64 `db:"id"                     json:"id"`

	// CodeHash is the hashed authorization code. The plaintext code is
	// only ever returned to the client once (in the redirect) and never
	// stored.
	CodeHash string `db:"code_hash" json:"-"`

	ClientID string `db:"client_id"       json:"clientId"`
	UserID   int64  `db:"user_id"         json:"-"`

	RedirectURI string `db:"redirect_uri" json:"redirectUri"`

	// Scopes: space-separated wire format (e.g. "openid profile email").
	Scopes string `db:"scopes" json:"scopes"`

	// PKCE fields (RFC 7636). Required for public clients, recommended
	// for all clients per OAuth 2.1.
	CodeChallenge       string `db:"code_challenge"        json:"codeChallenge"`
	CodeChallengeMethod string `db:"code_challenge_method" json:"codeChallengeMethod"`

	// Nonce is the OIDC nonce claim, echoed into the ID token to bind the
	// token to the original authorization request.
	Nonce string `db:"nonce" json:"nonce"`

	ExpiresAt time.Time `db:"expires_at" json:"expiresAt"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

// NewOAuthAuthorization returns a new OAuthAuthorization with a generated ID.
func NewOAuthAuthorization() *OAuthAuthorization {
	return &OAuthAuthorization{}
}

// IsExpired reports whether the authorization code has passed its expiry.
func (a *OAuthAuthorization) IsExpired() bool {
	return time.Now().After(a.ExpiresAt)
}

// ScopeList splits the space-separated scopes string into a slice.
func (a *OAuthAuthorization) ScopeList() []string {
	if a.Scopes == "" {
		return nil
	}
	return strings.Fields(a.Scopes)
}

// RequiresPKCE reports whether the authorization was initiated with PKCE.
// When true, /token must verify the code_verifier against CodeChallenge.
func (a *OAuthAuthorization) RequiresPKCE() bool {
	return a.CodeChallenge != ""
}

// Sentinel errors for authorization code operations.
var (
	ErrAuthorizationNotFound      = errors.New("authorization code not found")
	ErrAuthorizationExpired       = errors.New("authorization code expired")
	ErrAuthorizationRedirectMis   = errors.New("redirect uri mismatch")
	ErrAuthorizationClientMis     = errors.New("client id mismatch")
	ErrPKCEVerificationFailed     = errors.New("pkce verification failed")
	ErrPKCERequired               = errors.New("pkce required but code_verifier not provided")
	ErrUnsupportedChallengeMethod = errors.New("unsupported code challenge method")
)
