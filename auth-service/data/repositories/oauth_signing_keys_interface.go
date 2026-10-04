package repositories

import (
	"auth/data/entities"
)

type OAuthSigningKeysInterface interface {
	// Browse returns ALL signing keys (including inactive and expired) for
	// the admin UI. Excludes private key material.
	Browse() ([]*entities.OAuthSigningKey, error)

	// ReadByID returns a single signing key by its bigint id (with private key).
	ReadByID(id int64) (*entities.OAuthSigningKey, error)

	// ReadByKeyID returns a signing key by its JWT kid (with private key).
	ReadByKeyID(keyID string) (*entities.OAuthSigningKey, error)

	// ReadActiveSigningKey returns the current active signing key (with private key).
	ReadActiveSigningKey() (*entities.OAuthSigningKey, error)

	// BrowseVerificationKeys returns all non-expired signing keys for
	// JWKS, without private keys.
	BrowseVerificationKeys() ([]*entities.OAuthSigningKey, error)

	// Add inserts a new signing key into the database.
	Add(key *entities.OAuthSigningKey) (*entities.OAuthSigningKey, error)

	// SetActive flips a signing key's active flag. Atomic guard against
	// deactivating the last active row prevents the signer from ending
	// up with no key to mint with — returns ErrCannotDeactivateLastKey
	// in that case. Used by the admin Edit endpoint.
	SetActive(id int64, active bool) error

	// Deactivate marks a signing key as inactive. Thin wrapper over
	// SetActive(id, false) preserved for backward compatibility with
	// existing rotation callers.
	Deactivate(id int64) error

	// EditPrivateKey replaces the stored private key of one row.
	EditPrivateKey(id int64, privateKey string) error

	// Delete permanently removes a signing key.
	Delete(id int64) error

	// DeleteExpired removes keys that are both inactive and past expiry.
	DeleteExpired() (int64, error)
}
