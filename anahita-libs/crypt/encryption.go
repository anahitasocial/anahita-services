package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Credits to https://gist.github.com/fracasula/38aa1a4e7481f9cedfa78a0cdd5f1865 for the encryption/decryption code

type Encryption struct{}

// NewEncryption creates a new encryption instance
func NewEncryption() *Encryption {
	return &Encryption{}
}

// Encrypt encrypts a string value using a key
func (e *Encryption) Encrypt(key []byte, value string) (string, error) {
	byteMsg := []byte(value)
	block, err := aes.NewCipher(key)

	if err != nil {
		return "", fmt.Errorf("could not create new cipher: %v", err)
	}

	cipherText := make([]byte, aes.BlockSize+len(byteMsg))
	iv := cipherText[:aes.BlockSize]

	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("could not encrypt: %v", err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], byteMsg)

	return base64.StdEncoding.EncodeToString(cipherText), nil
}

// Decrypt decrypts a string value using a key
func (e *Encryption) Decrypt(key []byte, value string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(value)

	if err != nil {
		return "", fmt.Errorf("could not base64 decode: %v", err)
	}

	block, err := aes.NewCipher(key)

	if err != nil {
		return "", fmt.Errorf("could not create new cipher: %v", err)
	}

	if len(cipherText) < aes.BlockSize {
		return "", fmt.Errorf("invalid ciphertext block size")
	}

	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(cipherText, cipherText)

	return string(cipherText), nil
}
