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

	// RevokedAt is when Revoked was set, or nil for a token that is still
	// active — or that was revoked before the column existed.
	//
	// Reuse detection uses it to tell a rotation race from a replay: a
	// token revoked moments ago is a concurrent refresh, one revoked long
	// ago is an attacker. A nil value on a revoked row means the timestamp
	// predates the column, and is read as "long ago". Only a rotated token
	// can trip the cascade at all: see Rotated.
	RevokedAt *time.Time `db:"revoked_at" json:"revokedAt,omitempty"`

	// Rotated is whether it was revoked by being exchanged for a new pair,
	// and not by signing out, a client's deactivation or a reuse cascade.
	//
	// Only a rotated token presented again is a replay: it was spent, so
	// whoever holds it now holds a copy. A token revoked any other way was
	// never spent, and presenting it is a session that was ended, often by
	// a cascade elsewhere: refused, and no more. Cascading on it signed the
	// person out on every device, over and over (banu.io, 2026-11-02).
	// Rows revoked before the column existed read as not rotated.
	Rotated bool `db:"rotated" json:"-"`
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

	// ErrRefreshTokenRaced means another in-flight request rotated this
	// token first. It is NOT reuse: the token was valid when this request
	// read it, and both requests are legitimate holders of the same
	// plaintext. It carries no cascade revocation, because cascading here
	// would revoke the token pair the winning request just minted and
	// sign out a user who did nothing wrong.
	//
	// A client seeing this should re-read its token store rather than
	// retry with the same plaintext — the winner has already persisted a
	// fresh pair.
	ErrRefreshTokenRaced = errors.New("refresh token rotation raced")
)
