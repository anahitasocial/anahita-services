package entities

import (
	"errors"
	"strings"
	"time"
)

// OAuthRefreshToken is a long-lived token used to mint new access tokens
// without re-prompting the user. Stored hashed — plaintext returned once
// on token issuance.
type OAuthRefreshToken struct {
	ID int64 `db:"id"                     json:"id"`

	TokenHash string `db:"token_hash" json:"-"`

	UserID   int64  `db:"user_id"         json:"-"`
	ClientID string `db:"client_id"       json:"clientId"`

	// Scopes: space-separated wire format. Carried forward from the
	// authorization that originated this token.
	Scopes string `db:"scopes" json:"scopes"`

	Revoked   bool      `db:"revoked"    json:"revoked"`
	ExpiresAt time.Time `db:"expires_at" json:"expiresAt"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

// NewOAuthRefreshToken returns a new OAuthRefreshToken with a generated UUID.
func NewOAuthRefreshToken() *OAuthRefreshToken {
	return &OAuthRefreshToken{}
}

// IsValid reports whether the refresh token may still be used: not revoked
// and not expired.
func (t *OAuthRefreshToken) IsValid() bool {
	return !t.Revoked && !t.IsExpired()
}

// IsExpired reports whether the refresh token has passed its expiry.
func (t *OAuthRefreshToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// ScopeList splits the space-separated scopes string into a slice.
func (t *OAuthRefreshToken) ScopeList() []string {
	if t.Scopes == "" {
		return nil
	}
	return strings.Fields(t.Scopes)
}

// Sentinel errors for refresh token operations.
var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenRevoked  = errors.New("refresh token revoked")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
	ErrRefreshTokenReuse    = errors.New("refresh token reuse detected")
)
