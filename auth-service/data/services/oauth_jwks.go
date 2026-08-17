package services

import (
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	"auth/data/entities"
	"auth/internal/signing"
)

// getActiveSigningKey loads the current active signing key and parses its
// private key PEM. Called per-signing operation so that key rotation takes
// effect immediately.
func (s *OAuthService) getActiveSigningKey() (*entities.OAuthSigningKey, *rsa.PrivateKey, error) {
	signingKey, err := s.Repos.OAuthSigningKeys.ReadActiveSigningKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load active signing key: %w", err)
	}

	privateKey, err := signing.ParseRSAPrivateKeyFromPEM(signingKey.PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse active signing key: %w", err)
	}

	return signingKey, privateKey, nil
}

// getVerificationKey loads a signing key by its kid and parses its public
// key PEM. Used during JWT verification to find the key that signed a
// specific token.
func (s *OAuthService) getVerificationKey(kid string) (*rsa.PublicKey, error) {
	signingKey, err := s.Repos.OAuthSigningKeys.ReadByKeyID(kid)
	if err != nil {
		return nil, fmt.Errorf("signing key not found: %w", err)
	}

	if signingKey.IsExpired() {
		return nil, errors.New("signing key expired")
	}

	publicKey, err := signing.ParseRSAPublicKeyFromPEM(signingKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return publicKey, nil
}

// GetJWKS returns the complete JSON Web Key Set for the /.well-known/jwks.json
// endpoint. Includes all non-expired keys (both active and inactive) so
// that tokens signed by recently-rotated-out keys still verify until the
// key's natural expiry.
func (s *OAuthService) GetJWKS() (map[string]any, error) {
	keys, err := s.Repos.OAuthSigningKeys.BrowseVerificationKeys()
	if err != nil {
		return nil, err
	}

	jwks := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		jwk, err := s.BuildJWK(key)
		if err != nil {
			log.Printf("oauth: skipping malformed signing key %s: %v", key.KeyID, err)
			continue
		}
		jwks = append(jwks, jwk)
	}

	return map[string]any{"keys": jwks}, nil
}

// BuildJWK converts a signing key entity into a JWK map per RFC 7517.
func (s *OAuthService) BuildJWK(key *entities.OAuthSigningKey) (map[string]any, error) {
	rsaPub, err := signing.ParseRSAPublicKeyFromPEM(key.PublicKey)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"kty": "RSA",
		"use": "sig",
		"alg": key.Algorithm,
		"kid": key.KeyID,
		"n":   base64.RawURLEncoding.EncodeToString(rsaPub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaPub.E)).Bytes()),
	}, nil
}

// RotateSigningKey generates a new RSA keypair, adds it to the repository
// as active, and deactivates the previous active key. The deactivated key
// remains in the database (and in JWKS) until its natural expiry so that
// tokens signed by it still verify.
//
// newKeyLifetime is how long the new key will remain valid for signing AND
// verification. A reasonable value is 90 days.
func (s *OAuthService) RotateSigningKey(newKeyLifetime time.Duration) (*entities.OAuthSigningKey, error) {
	priv, err := signing.GenerateRSAKeyPair(2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	privatePEM := signing.MarshalRSAPrivateKeyToPEM(priv)
	publicPEM, err := signing.MarshalRSAPublicKeyToPEM(&priv.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	kid, err := signing.GenerateKeyID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate kid: %w", err)
	}

	newKey := entities.NewOAuthSigningKey()
	newKey.KeyID = kid
	newKey.PrivateKey = privatePEM
	newKey.PublicKey = publicPEM
	newKey.ExpiresAt = time.Now().Add(newKeyLifetime)

	// Deactivate the previous active key, if any. Swallow ErrNoActiveSigningKey
	// — first-time rotation has nothing to deactivate.
	previous, err := s.Repos.OAuthSigningKeys.ReadActiveSigningKey()
	if err == nil {
		if err := s.Repos.OAuthSigningKeys.Deactivate(previous.ID); err != nil {
			return nil, fmt.Errorf("failed to deactivate previous signing key: %w", err)
		}
	}

	persisted, err := s.Repos.OAuthSigningKeys.Add(newKey)
	if err != nil {
		return nil, fmt.Errorf("failed to persist new signing key: %w", err)
	}

	log.Printf("oauth: rotated signing key, new kid=%s", persisted.KeyID)
	return persisted, nil
}
