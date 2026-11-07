package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"testing"
)

const sessionKey = "12345678901234567890123456789012"

// TestEncryption tests the encryption and decryption of a string
func TestEncryption(t *testing.T) {
	encryption := &Encryption{}
	plaintext := "Hello World!"

	ciphertext, err := encryption.Encrypt([]byte(sessionKey), plaintext)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := encryption.Decrypt([]byte(sessionKey), ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != decrypted {
		t.Errorf("Expected %s, got %s", plaintext, decrypted)
	}

	// Each sealing has a nonce of its own: the same value twice is not the
	// same text twice.
	again, _ := encryption.Encrypt([]byte(sessionKey), plaintext)
	if again == ciphertext {
		t.Error("the same value sealed twice gave the same text")
	}
}

// A value changed by one bit is refused, where CFB read it as something
// else (roadmap row 119).
func TestEncryptionRefusesAChangedValue(t *testing.T) {
	encryption := &Encryption{}
	sealed, err := encryption.Encrypt([]byte(sessionKey), `{"user_id":5}`)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(sealed)
	raw[len(raw)-1] ^= 1
	changed := base64.StdEncoding.EncodeToString(raw)

	if _, err := encryption.Decrypt([]byte(sessionKey), changed); !errors.Is(err, ErrUnreadable) {
		t.Errorf("a changed value: got %v, want ErrUnreadable", err)
	}
}

// Sealed under another key, cut short, or not base64 at all: refused.
func TestEncryptionRefusesWhatItDidNotSeal(t *testing.T) {
	encryption := &Encryption{}
	sealed, _ := encryption.Encrypt([]byte(sessionKey), "value")

	otherKey := "abcdefghijabcdefghijabcdefghijab"
	for name, value := range map[string]string{
		"another key": sealed,
		"cut short":   sealed[:8],
		"not base64":  "%%%",
	} {
		key := sessionKey
		if name == "another key" {
			key = otherKey
		}
		if _, err := encryption.Decrypt([]byte(key), value); !errors.Is(err, ErrUnreadable) {
			t.Errorf("%s: got %v, want ErrUnreadable", name, err)
		}
	}
}

// A session sealed with AES-CFB before v1.0.17 is refused, not read as
// garbage: its holder signs in again.
func TestEncryptionRefusesTheCipherBefore(t *testing.T) {
	block, _ := aes.NewCipher([]byte(sessionKey))
	value := []byte(`{"user_id":5}`)
	old := make([]byte, aes.BlockSize+len(value))
	copy(old[:aes.BlockSize], "0123456789abcdef")
	cipher.NewCFBEncrypter(block, old[:aes.BlockSize]).XORKeyStream(old[aes.BlockSize:], value)

	encryption := &Encryption{}
	if _, err := encryption.Decrypt([]byte(sessionKey), base64.StdEncoding.EncodeToString(old)); !errors.Is(err, ErrUnreadable) {
		t.Errorf("a CFB value: got %v, want ErrUnreadable", err)
	}
}
