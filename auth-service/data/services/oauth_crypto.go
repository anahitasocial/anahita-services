package services

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// This file holds OAuth-flow crypto helpers: random secret generation,
// hashing for storage, and constant-time comparison. RSA / PEM / kid
// helpers used by the signer have moved to auth-service/internal/signing
// (see signer_keys.go) — they're crypto primitives the signer needs,
// not OAuth flow logic.

// GenerateClientSecret generates a random OAuth client secret (plaintext).
// Callers should hash via HashToken before storing.
func GenerateClientSecret() (string, error) {
	return generateRandomToken(32)
}

// generateRandomToken returns a URL-safe random token of the given
// byte length, base64-encoded without padding.
func generateRandomToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns the base64-encoded SHA-256 hash of the given token.
// Used for hashing refresh tokens, authorization codes, and client
// secrets before storing them. SHA-256 is appropriate here (not
// bcrypt) because these values are high-entropy random strings.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// ConstantTimeEqual compares two strings in constant time to prevent
// timing attacks on secret/hash comparison.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
