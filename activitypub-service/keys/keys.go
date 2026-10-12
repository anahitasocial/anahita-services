// Package keys makes and reads the key a person here signs with.
//
// Whatever is sent to another server in somebody's name is signed with a
// key of theirs, and the other server checks it against the public half,
// which it reads from the person's description. So the public half is told
// to anybody, and the private half is never out of this service: it is kept
// encrypted, and read only to sign.
package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

// Algorithm is the one kind of key made. RSA with SHA-256 is what every
// server that speaks ActivityPub understands; 2048 bits is what they make
// themselves.
const (
	Algorithm = "rsa-2048"
	bits      = 2048
)

// Pair is a key as it is kept: both halves, as PEM.
type Pair struct {
	PublicPEM  string
	PrivatePEM string
}

// Generate makes a new key.
func Generate() (*Pair, error) {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}

	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}

	return &Pair{
		PublicPEM:  string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public})),
		PrivatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})),
	}, nil
}

// ParsePrivate reads the private half back, to sign with.
func ParsePrivate(privatePEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil {
		return nil, errors.New("no key in what was kept")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("what was kept is not an RSA key")
	}
	return key, nil
}

// ParsePublic reads a public half: ours, or one another server sent.
func ParsePublic(publicPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicPEM))
	if block == nil {
		return nil, errors.New("no key in what was sent")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Some servers send the older form.
		if older, olderErr := x509.ParsePKCS1PublicKey(block.Bytes); olderErr == nil {
			return older, nil
		}
		return nil, err
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("what was sent is not an RSA key")
	}
	return key, nil
}
