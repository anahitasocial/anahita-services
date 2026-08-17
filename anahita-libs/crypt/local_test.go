package crypt

import (
	"errors"
	"strings"
	"testing"

	"anahita-libs/constants"
)

const testKey = "12345678901234567890123456789012"

type stubConfig struct {
	backend  string
	localKey string
	kmsKeyId string
}

func (c stubConfig) GetCryptBackend() string  { return c.backend }
func (c stubConfig) GetCryptLocalKey() string { return c.localKey }
func (c stubConfig) GetAwsKmsKeyId() string   { return c.kmsKeyId }

func TestLocalCryptRoundTrip(t *testing.T) {
	c, err := NewLocalCrypt(testKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plaintext := "JBSWY3DPEHPK3PXP"

	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ciphertext == plaintext {
		t.Fatal("ciphertext should not equal plaintext")
	}

	got, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plaintext {
		t.Errorf("expected %q, got %q", plaintext, got)
	}
}

// Two encryptions of the same plaintext must differ — a fresh nonce per
// call is what keeps identical TOTP secrets from being correlatable in
// the database.
func TestLocalCryptNonceIsFresh(t *testing.T) {
	c, _ := NewLocalCrypt(testKey)

	first, err := c.Encrypt("same")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	second, err := c.Encrypt("same")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if first == second {
		t.Error("expected different ciphertexts for repeated encryption")
	}
}

func TestLocalCryptRejectsBadKey(t *testing.T) {
	if _, err := NewLocalCrypt(""); !errors.Is(err, ErrLocalKeyMissing) {
		t.Errorf("expected ErrLocalKeyMissing, got %v", err)
	}
	if _, err := NewLocalCrypt("too-short"); !errors.Is(err, ErrLocalKeySize) {
		t.Errorf("expected ErrLocalKeySize, got %v", err)
	}
}

// Tampered ciphertext must fail rather than decrypt to garbage — this
// is the property CFB mode doesn't give us.
func TestLocalCryptRejectsTamperedCiphertext(t *testing.T) {
	c, _ := NewLocalCrypt(testKey)

	ciphertext, err := c.Encrypt("secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	tampered := []byte(ciphertext)
	// Flip a character near the end, inside the tag/payload rather than
	// the base64 padding.
	idx := len(tampered) - 4
	if tampered[idx] == 'A' {
		tampered[idx] = 'B'
	} else {
		tampered[idx] = 'A'
	}

	if _, err := c.Decrypt(string(tampered)); err == nil {
		t.Error("expected error decrypting tampered ciphertext")
	}
}

func TestLocalCryptRejectsShortCiphertext(t *testing.T) {
	c, _ := NewLocalCrypt(testKey)

	if _, err := c.Decrypt("YQ=="); !errors.Is(err, ErrLocalCiphertext) {
		t.Errorf("expected ErrLocalCiphertext, got %v", err)
	}
}

// A value encrypted under one key must not decrypt under another.
func TestLocalCryptRejectsWrongKey(t *testing.T) {
	a, _ := NewLocalCrypt(testKey)
	b, _ := NewLocalCrypt("abcdefghijklmnopqrstuvwxyz012345")

	ciphertext, err := a.Encrypt("secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if _, err := b.Decrypt(ciphertext); err == nil {
		t.Error("expected error decrypting with a different key")
	}
}

func TestNewCryptSelectsBackend(t *testing.T) {
	local, err := NewCrypt(stubConfig{backend: constants.CryptBackendLocal, localKey: testKey})
	if err != nil {
		t.Fatalf("unexpected error for local backend: %v", err)
	}
	if _, ok := local.(*localCrypt); !ok {
		t.Errorf("expected *localCrypt, got %T", local)
	}

	kms, err := NewCrypt(stubConfig{backend: constants.CryptBackendKMS, kmsKeyId: "key-id"})
	if err != nil {
		t.Fatalf("unexpected error for kms backend: %v", err)
	}
	if _, ok := kms.(*kmsCrypt); !ok {
		t.Errorf("expected *kmsCrypt, got %T", kms)
	}
}

func TestNewCryptUnknownBackend(t *testing.T) {
	_, err := NewCrypt(stubConfig{backend: "vault"})
	if err == nil || !strings.Contains(err.Error(), "vault") {
		t.Errorf("expected error naming the unknown backend, got %v", err)
	}
}

// The local backend must not require an AWS KMS key id — that coupling
// is the thing this backend exists to remove.
func TestNewCryptLocalDoesNotNeedKmsKeyId(t *testing.T) {
	if _, err := NewCrypt(stubConfig{backend: constants.CryptBackendLocal, localKey: testKey}); err != nil {
		t.Errorf("local backend should build without a KMS key id, got %v", err)
	}
}
