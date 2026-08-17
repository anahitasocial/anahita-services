package crypt

import (
	"testing"
)

// TestEncryption tests the encryption and decryption of a string
func TestEncryption(t *testing.T) {
	encryption := &Encryption{}
	key := "12345678901234567890123456789012"
	plaintext := "Hello World!"

	ciphertext, err := encryption.Encrypt([]byte(key), plaintext)

	if err != nil {
		t.Error(err)
	}

	decrypted, err := encryption.Decrypt([]byte(key), ciphertext)

	if err != nil {
		t.Error(err)
	}

	if plaintext != decrypted {
		t.Errorf("Expected %s, got %s", plaintext, decrypted)
	}
}
