package entities

import (
	"errors"
	"time"

	"anahita-libs/constants"
)

// OAuth signing algorithm constants. RS256 is the OIDC default. Others
// defined for forward compatibility.
const (
	SigningAlgRS256 = "RS256"
	SigningAlgRS384 = "RS384"
	SigningAlgRS512 = "RS512"
	SigningAlgES256 = "ES256"
	SigningAlgES384 = "ES384"
)

// OAuthSigningKey is an asymmetric keypair used to sign JWT access tokens
// and ID tokens. Keys are rotated on a schedule; multiple keys can be
// active simultaneously to support graceful rotation (old keys verify
// existing tokens while new keys sign new tokens).
//
// The public key is exposed via the JWKS endpoint (/.well-known/jwks.json).
// Downstream microservices fetch and cache JWKS to verify tokens without
// calling auth-service on every request.
type OAuthSigningKey struct {
	ID int64 `db:"id"                   json:"id"`

	// KeyID is the JWT "kid" header value. Clients use it to select the
	// correct public key from JWKS when verifying a token.
	KeyID string `db:"key_id" json:"keyId"`

	Algorithm string `db:"algorithm" json:"algorithm"`

	// PublicKey is PEM-encoded. Exposed via JWKS after conversion to JWK.
	PublicKey string `db:"public_key" json:"-"`

	// PrivateKey is PEM-encoded. Must never leave the auth-service process.
	// Consider encrypting at rest via KMS for production.
	PrivateKey string `db:"private_key" json:"-"`

	Active    bool      `db:"active"     json:"active"`
	ExpiresAt time.Time `db:"expires_at" json:"expiresAt"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`

	Backend string `db:"backend" json:"backend"`
}

// NewOAuthSigningKey returns a new OAuthSigningKey with sensible defaults.
func NewOAuthSigningKey() *OAuthSigningKey {
	return &OAuthSigningKey{
		Algorithm: SigningAlgRS256,
		Active:    true,
		Backend:   constants.OAuthSignerBackendLocal,
	}
}

// IsUsableForSigning reports whether this key should be used to sign new
// tokens: active and not past its expiry.
func (k *OAuthSigningKey) IsUsableForSigning() bool {
	return k.Active && !k.IsExpired()
}

// IsExpired reports whether the key has passed its expiry.
func (k *OAuthSigningKey) IsExpired() bool {
	return time.Now().After(k.ExpiresAt)
}

// Sentinel errors for signing key operations.
var (
	ErrSigningKeyNotFound = errors.New("signing key not found")
	ErrSigningKeyExpired  = errors.New("signing key expired")
	ErrSigningKeyInactive = errors.New("signing key inactive")
	ErrNoActiveSigningKey = errors.New("no active signing key available")
)
