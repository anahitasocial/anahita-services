package services

import (
	"encoding/base64"
	"math/big"

	"auth/data/entities"
	"auth/internal/signing"
)

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
