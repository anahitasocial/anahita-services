package crypt

import (
	"fmt"

	"anahita-libs/constants"
)

// NewCrypt returns a Crypt matching the configured backend.
//
// "local" (the default) encrypts with an AES-256 key from config and
// has no cloud dependency. "kms" delegates to AWS KMS and needs both a
// key id and credentials resolvable by the AWS SDK.
//
// Call this once at startup rather than per request: it fails loudly on
// a misconfigured key, and that failure belongs at boot, not in the
// middle of somebody's 2FA enrolment.
func NewCrypt(config ConfigInterface) (Crypt, error) {
	switch config.GetCryptBackend() {
	case constants.CryptBackendLocal:
		return NewLocalCrypt(config.GetCryptLocalKey())
	case constants.CryptBackendKMS:
		return NewKmsCrypt(config), nil
	default:
		return nil, fmt.Errorf("crypt: unknown backend %q", config.GetCryptBackend())
	}
}
