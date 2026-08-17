package signing

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

const (
	PEMBlockTypeRSAPrivateKey = "RSA PRIVATE KEY" // PKCS#1
	PEMBlockTypePrivateKey    = "PRIVATE KEY"     // PKCS#8
	PEMBlockTypeRSAPublicKey  = "RSA PUBLIC KEY"  // PKCS#1 public
	PEMBlockTypePublicKey     = "PUBLIC KEY"      // PKIX public
)

// ParseRSAPrivateKeyFromPEM parses a PEM-encoded RSA private key. Accepts
// both PKCS#1 and PKCS#8 encodings.
func ParseRSAPrivateKeyFromPEM(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	switch block.Type {
	case PEMBlockTypeRSAPrivateKey:
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case PEMBlockTypePrivateKey:
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}

		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}

		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}

// ParseRSAPublicKeyFromPEM parses a PEM-encoded RSA public key. Accepts
// both PKCS#1 and PKIX encodings.
func ParseRSAPublicKeyFromPEM(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	switch block.Type {
	case PEMBlockTypeRSAPublicKey:
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case PEMBlockTypePublicKey:
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}

		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not an RSA public key")
		}

		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}

// MarshalRSAPrivateKeyToPEM encodes an RSA private key to PEM (PKCS#1).
func MarshalRSAPrivateKeyToPEM(key *rsa.PrivateKey) string {
	block := &pem.Block{
		Type:  PEMBlockTypeRSAPrivateKey,
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return string(pem.EncodeToMemory(block))
}

// MarshalRSAPublicKeyToPEM encodes an RSA public key to PEM (PKIX).
func MarshalRSAPublicKeyToPEM(key *rsa.PublicKey) (string, error) {
	bytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", err
	}
	block := &pem.Block{
		Type:  PEMBlockTypePublicKey,
		Bytes: bytes,
	}
	return string(pem.EncodeToMemory(block)), nil
}

// GenerateRSAKeyPair generates a new RSA key pair for signing. Rejects
// key sizes below 2048 bits.
func GenerateRSAKeyPair(bits int) (*rsa.PrivateKey, error) {
	if bits < 2048 {
		return nil, errors.New("key size must be at least 2048 bits")
	}
	return rsa.GenerateKey(rand.Reader, bits)
}

// GenerateKeyID generates a random key ID for the JWT "kid" header.
func GenerateKeyID() (string, error) {
	return generateRandomToken(16)
}

// GenerateClientSecret generates a random OAuth client secret (plaintext).
// Callers should hash via HashToken before storing.
func GenerateClientSecret() (string, error) {
	return generateRandomToken(32)
}

// generateRandomToken returns a URL-safe random token of the given byte
// length, base64-encoded without padding.
func generateRandomToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns the base64-encoded SHA-256 hash of the given token.
// Used for hashing refresh tokens, authorization codes, and client secrets
// before storing them. SHA-256 is appropriate here (not bcrypt) because
// these values are high-entropy random strings.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// ConstantTimeEqual compares two strings in constant time to prevent
// timing attacks on secret/hash comparison.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
