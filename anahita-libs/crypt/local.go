package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"io"
)

// LocalCryptKeySize is the required length in bytes of the local
// backend's key. AES-256 — same size as SECURE_COOKIE_KEY, so the
// operational habit of generating a 32-byte secret carries over.
const LocalCryptKeySize = 32

var (
	// ErrLocalKeyMissing is returned when the local backend is selected
	// without a key configured.
	ErrLocalKeyMissing = errors.New("crypt: local key is missing")
	// ErrLocalKeySize is returned when the configured key is not
	// exactly LocalCryptKeySize bytes.
	ErrLocalKeySize = errors.New("crypt: local key must be 32 bytes")
	// ErrLocalCiphertext is returned when a ciphertext is too short to
	// contain a nonce, i.e. it was not produced by this backend.
	ErrLocalCiphertext = errors.New("crypt: ciphertext is malformed")
)

// localCrypt encrypts with AES-256-GCM using a key from configuration.
//
// GCM rather than the CFB mode in Encryption: this data is at rest in
// the database and unauthenticated ciphertext there can be tampered
// with undetectably. TOTP secrets in particular must fail closed on
// modification rather than decrypt to garbage.
//
// Ciphertexts are NOT interchangeable with the KMS backend's. Switching
// backends leaves previously stored values undecryptable — anything
// already encrypted has to be re-encrypted, or in the case of TOTP
// secrets, re-enrolled.
type localCrypt struct {
	aead cipher.AEAD
}

// NewLocalCrypt returns a Crypt backed by the given AES-256 key, which
// must be exactly LocalCryptKeySize bytes.
func NewLocalCrypt(key string) (Crypt, error) {
	if key == "" {
		return nil, ErrLocalKeyMissing
	}
	if len(key) != LocalCryptKeySize {
		return nil, fmt.Errorf("%w, got %d", ErrLocalKeySize, len(key))
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("crypt: could not create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypt: could not create GCM: %w", err)
	}

	return &localCrypt{aead: aead}, nil
}

// Encrypt returns base64(nonce || ciphertext || tag). A fresh random
// nonce is generated per call — GCM nonce reuse under the same key
// leaks plaintext, so this must never be derived from the message.
func (crypt *localCrypt) Encrypt(text string) (string, error) {
	nonce := make([]byte, crypt.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypt: could not generate nonce: %w", err)
	}

	sealed := crypt.aead.Seal(nonce, nonce, []byte(text), nil)

	return b64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. Returns an error rather than garbage if the
// ciphertext was modified, truncated, or produced under a different key.
func (crypt *localCrypt) Decrypt(text string) (string, error) {
	blob, err := b64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", fmt.Errorf("crypt: failed to decode base64 string: %w", err)
	}

	nonceSize := crypt.aead.NonceSize()
	if len(blob) < nonceSize {
		return "", ErrLocalCiphertext
	}

	nonce, sealed := blob[:nonceSize], blob[nonceSize:]

	plaintext, err := crypt.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("crypt: failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}
