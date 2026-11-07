package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Encryption seals the values sessions keep, in a cookie or in Redis, with
// AES-GCM: a value is hidden, and a value that was changed, or sealed under
// another key, is refused rather than read as something else (roadmap row
// 119). Until v1.0.17 this was AES-CFB, which hid a value and noticed
// nothing; a value sealed that way is refused now, so everybody signed in
// then signs in once more.
//
// What is kept for longer, as TOTP secrets and signing keys, goes through
// Crypt (local.go, kms.go), not this.
type Encryption struct{}

// ErrUnreadable is a value that cannot be opened: changed, cut short, or
// sealed under another key or the cipher before.
var ErrUnreadable = errors.New("crypt: value cannot be read")

// NewEncryption creates a new encryption instance.
func NewEncryption() *Encryption {
	return &Encryption{}
}

// Encrypt seals a value with the key: a nonce of its own, then the sealed
// value with its tag, base64 encoded.
func (e *Encryption) Encrypt(key []byte, value string) (string, error) {
	aead, err := gcm(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("could not encrypt: %v", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(value), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt opens a value Encrypt sealed with the same key. Anything else is
// ErrUnreadable.
func (e *Encryption) Decrypt(key []byte, value string) (string, error) {
	aead, err := gcm(key)
	if err != nil {
		return "", err
	}
	sealed, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", ErrUnreadable
	}
	if len(sealed) < aead.NonceSize()+aead.Overhead() {
		return "", ErrUnreadable
	}
	nonce, body := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	opened, err := aead.Open(nil, nonce, body, nil)
	if err != nil {
		return "", ErrUnreadable
	}
	return string(opened), nil
}

func gcm(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("could not create new cipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("could not create GCM: %v", err)
	}
	return aead, nil
}
