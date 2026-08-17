package signing

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"fmt"

	"auth/data/entities"
	"auth/data/repositories"
)

// SignerLocal signs JWTs using a private key stored in the
// oauth_signing_keys table. Used in development, CI, and any
// deployment that doesn't have access to AWS KMS.
//
// The signer is constructed for a single algorithm and rejects keys
// of any other algorithm. Algorithm migrations (e.g. RS256 → ES256)
// happen by deploying a new signer pointed at keys of the new
// algorithm, never at runtime within a single signer instance.
type SignerLocal struct {
	repo repositories.OAuthSigningKeysInterface
	alg  string
}

// NewSignerLocal returns a Signer that reads the active signing key
// from the given repository on each operation.
//
// alg must match the algorithm of any key the repository returns.
// Currently only RS256 is supported in local mode — when ES256 keys
// are first provisioned, this will be extended. Returns
// ErrUnsupportedAlgorithm if alg is not recognized.
func NewSignerLocal(repo repositories.OAuthSigningKeysInterface, alg string) (*SignerLocal, error) {
	if alg == "" {
		return nil, errors.New("signing: algorithm required")
	}
	if alg != entities.SigningAlgRS256 {
		return nil, fmt.Errorf("%w: local signer currently supports only RS256, got %q",
			ErrUnsupportedAlgorithm, alg)
	}
	return &SignerLocal{repo: repo, alg: alg}, nil
}

// Algorithm returns the JWA algorithm name this signer produces.
// Static for the lifetime of the signer.
func (s *SignerLocal) Algorithm() string {
	return s.alg
}

// ActiveKeyID returns the kid of the current active signing key.
// Reads from the database on each call — local-mode database hits
// are sub-millisecond and not on a critical path that needs caching.
func (s *SignerLocal) ActiveKeyID(ctx context.Context) (string, error) {
	key, err := s.loadActiveKey()
	if err != nil {
		return "", err
	}
	return key.KeyID, nil
}

// Sign reads the active key, parses its private key from PEM, and
// signs the JWT signing input using PKCS#1 v1.5 with SHA-256
// (the RS256 specification).
func (s *SignerLocal) Sign(ctx context.Context, signingInput []byte) ([]byte, error) {
	key, err := s.loadActiveKey()
	if err != nil {
		return nil, err
	}

	privateKey, err := ParseRSAPrivateKeyFromPEM(key.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("signing: parse private key for kid=%s: %w", key.KeyID, err)
	}

	hashed := sha256.Sum256(signingInput)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return nil, fmt.Errorf("signing: rsa sign: %w", err)
	}

	return signature, nil
}

// loadActiveKey reads the current active signing key and verifies its
// algorithm matches the signer's configured algorithm. The algorithm
// check is the load-bearing defense against a misconfiguration where
// (e.g.) an ES256 key gets inserted into a table the RS256 signer
// reads from — without it, the signer would happily mint tokens with
// alg="RS256" but a signature only an ES256 verifier could check.
func (s *SignerLocal) loadActiveKey() (*entities.OAuthSigningKey, error) {
	key, err := s.repo.ReadActiveSigningKey()
	if errors.Is(err, entities.ErrSigningKeyNotFound) {
		return nil, ErrNoActiveSigningKey
	}
	if err != nil {
		return nil, fmt.Errorf("signing: read active key: %w", err)
	}
	if key.Algorithm != s.alg {
		return nil, fmt.Errorf("%w: signer alg=%s, active key alg=%s, kid=%s",
			ErrAlgorithmMismatch, s.alg, key.Algorithm, key.KeyID)
	}
	return key, nil
}
