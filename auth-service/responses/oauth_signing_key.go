package responses

import (
	"time"

	"auth/data/entities"
)

// OAuthSigningKeyItem is a key the installation signs tokens with, as an
// administrator sees it. Neither half of the key is sent: the public one
// is in the JWKS, and the private one never leaves the service.
type OAuthSigningKeyItem struct {
	// Its number here.
	ID int64 `json:"id"`
	// The `kid` a token names it by, in its header.
	KeyID string `json:"key_id"`
	// `RS256`.
	Algorithm string `json:"algorithm"`
	// Whether new tokens are signed with it. Tokens it signed are still
	// verified while it is in the JWKS.
	Active bool `json:"active"`
	// When it stops being signed with.
	ExpiresAt time.Time `json:"expires_at"`
	// When it was made.
	CreatedAt time.Time `json:"created_at"`
	// Where the private key is kept: `local`, in the database, or `kms`.
	Backend string `json:"backend"`
}

func MapOAuthSigningKey(key *entities.OAuthSigningKey) *OAuthSigningKeyItem {
	return &OAuthSigningKeyItem{
		ID:        key.ID,
		KeyID:     key.KeyID,
		Algorithm: key.Algorithm,
		Active:    key.Active,
		ExpiresAt: key.ExpiresAt,
		CreatedAt: key.CreatedAt,
		Backend:   key.Backend,
	}
}

func MapOAuthSigningKeys(keys []*entities.OAuthSigningKey) []*OAuthSigningKeyItem {
	items := make([]*OAuthSigningKeyItem, 0, len(keys))
	for _, key := range keys {
		items = append(items, MapOAuthSigningKey(key))
	}
	return items
}
