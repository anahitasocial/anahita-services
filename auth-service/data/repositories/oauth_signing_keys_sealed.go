package repositories

import (
	"errors"
	"fmt"
	"strings"

	"auth/data/entities"
)

// KeySealer encrypts and decrypts a signing key's private half. crypt.Crypt
// satisfies it, so the key is sealed with whatever protects the other secrets
// at rest: a local AES-256 key, or KMS.
type KeySealer interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

// sealedPrefix marks a stored private key as encrypted. Versioned, so the
// scheme can change without guessing what an old row holds.
const sealedPrefix = "sealed:v1:"

// pemPrefix is how a private key stored in the clear begins.
const pemPrefix = "-----BEGIN"

// ErrPrivateKeyUnreadable means a stored private key is neither sealed nor a
// PEM block, so nothing can be made of it.
var ErrPrivateKeyUnreadable = errors.New("signing key: stored private key is neither sealed nor PEM")

// SealedOAuthSigningKeys keeps the private half of a locally stored signing
// key encrypted in the database.
//
// It used to be stored as plain PEM. Anybody able to read the
// oauth_signing_keys table, through a backup, a replica or a SQL injection
// anywhere in the stack, could then mint a token for any account, and the
// tokens would verify. Sealed, the table alone is not enough: the reader
// also needs the key that protects secrets at rest, which is not in the
// database.
//
// It wraps the real repository rather than changing it, so the two writers
// (the seeder and key rotation) and the one reader that matters (the local
// signer) need no change, and cannot forget.
//
// KMS-backed rows have no private key here at all and pass through untouched.
type SealedOAuthSigningKeys struct {
	OAuthSigningKeysInterface
	sealer KeySealer
}

func NewSealedOAuthSigningKeys(inner OAuthSigningKeysInterface, sealer KeySealer) *SealedOAuthSigningKeys {
	return &SealedOAuthSigningKeys{OAuthSigningKeysInterface: inner, sealer: sealer}
}

// Add seals the private key on its way in. The key handed back carries the
// PEM again, as the caller gave it.
func (s *SealedOAuthSigningKeys) Add(key *entities.OAuthSigningKey) (*entities.OAuthSigningKey, error) {
	pem := key.PrivateKey

	sealed, err := s.seal(pem)
	if err != nil {
		return nil, err
	}

	stored := *key
	stored.PrivateKey = sealed

	added, err := s.OAuthSigningKeysInterface.Add(&stored)
	if err != nil {
		return nil, err
	}

	added.PrivateKey = pem
	return added, nil
}

func (s *SealedOAuthSigningKeys) ReadByID(id int64) (*entities.OAuthSigningKey, error) {
	return s.opened(s.OAuthSigningKeysInterface.ReadByID(id))
}

func (s *SealedOAuthSigningKeys) ReadByKeyID(keyID string) (*entities.OAuthSigningKey, error) {
	return s.opened(s.OAuthSigningKeysInterface.ReadByKeyID(keyID))
}

func (s *SealedOAuthSigningKeys) ReadActiveSigningKey() (*entities.OAuthSigningKey, error) {
	return s.opened(s.OAuthSigningKeysInterface.ReadActiveSigningKey())
}

// SealStored encrypts every private key still stored in the clear, and
// reports how many it changed. It is the upgrade for a database written
// before keys were sealed, and safe to run at every start: a sealed row is
// left alone.
func (s *SealedOAuthSigningKeys) SealStored() (int, error) {
	keys, err := s.OAuthSigningKeysInterface.Browse()
	if err != nil {
		return 0, err
	}

	sealedCount := 0
	for _, listed := range keys {
		// Browse leaves the private key out. Read the row itself, through
		// the wrapped repository so it comes back exactly as stored.
		key, err := s.OAuthSigningKeysInterface.ReadByID(listed.ID)
		if err != nil {
			return sealedCount, fmt.Errorf("signing key %d: %w", listed.ID, err)
		}

		if key.PrivateKey == "" || isSealed(key.PrivateKey) {
			continue
		}

		sealed, err := s.seal(key.PrivateKey)
		if err != nil {
			return sealedCount, fmt.Errorf("signing key %d: %w", key.ID, err)
		}

		if err := s.OAuthSigningKeysInterface.EditPrivateKey(key.ID, sealed); err != nil {
			return sealedCount, fmt.Errorf("signing key %d: %w", key.ID, err)
		}
		sealedCount++
	}

	return sealedCount, nil
}

func isSealed(stored string) bool {
	return strings.HasPrefix(stored, sealedPrefix)
}

// seal encrypts a PEM private key for storage. An empty key (a KMS row) and
// an already sealed one are returned as they are.
func (s *SealedOAuthSigningKeys) seal(privateKey string) (string, error) {
	if privateKey == "" || isSealed(privateKey) {
		return privateKey, nil
	}

	encrypted, err := s.sealer.Encrypt(privateKey)
	if err != nil {
		return "", fmt.Errorf("signing key: seal private key: %w", err)
	}

	return sealedPrefix + encrypted, nil
}

// opened turns a stored key back into one whose PrivateKey is PEM.
//
// A row still in the clear is returned as it is. SealStored converts those at
// start-up; accepting one here means a row that slipped past it is signed
// with rather than taking sign-in down.
func (s *SealedOAuthSigningKeys) opened(key *entities.OAuthSigningKey, err error) (*entities.OAuthSigningKey, error) {
	if err != nil {
		return nil, err
	}

	switch stored := key.PrivateKey; {
	case stored == "":
		return key, nil

	case isSealed(stored):
		pem, err := s.sealer.Decrypt(strings.TrimPrefix(stored, sealedPrefix))
		if err != nil {
			// Almost always the wrong key: CRYPT_LOCAL_KEY changed since
			// this row was written.
			return nil, fmt.Errorf("signing key %s: cannot open the private key, was the secrets key changed? %w", key.KeyID, err)
		}
		opened := *key
		opened.PrivateKey = pem
		return &opened, nil

	case strings.HasPrefix(stored, pemPrefix):
		return key, nil

	default:
		return nil, fmt.Errorf("signing key %s: %w", key.KeyID, ErrPrivateKeyUnreadable)
	}
}
