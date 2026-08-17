package signing

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"fmt"

	"auth/data/entities"
	"auth/data/repositories"
)

// VerifierLocal verifies JWT signatures using public keys read from
// the oauth_signing_keys table.
//
// Unlike SignerLocal, the verifier is NOT constructed for a single
// algorithm. It dispatches on the algorithm of the key matching kid,
// so it can verify in-flight tokens during an algorithm migration
// (e.g. RS256 tokens still circulating after ES256 became the active
// signing algorithm). The alg-confusion defense lives in the algorithm
// vs. key check, not in a constructor-level filter.
type VerifierLocal struct {
	repo repositories.OAuthSigningKeysInterface
}

// NewVerifierLocal returns a Verifier that resolves public keys from
// the given repository.
func NewVerifierLocal(repo repositories.OAuthSigningKeysInterface) *VerifierLocal {
	return &VerifierLocal{repo: repo}
}

// Verify implements the Verifier interface. The flow is:
//
//  1. Look up the key by kid. Unknown kid → ErrUnknownKID.
//  2. Check that the alg declared in the JWT header matches the alg
//     registered for that key. Mismatch → ErrAlgorithmMismatch.
//     This is the primary defense against alg-confusion attacks.
//  3. Dispatch on alg to the appropriate cryptographic verification.
//     Unsupported alg → ErrUnsupportedAlgorithm.
//  4. Run the verification. Failure → ErrInvalidSignature.
//
// Note: the verifier does NOT check key.Active or key.ExpiresAt. A
// key that is past its expiry but still in the table is a valid
// verification source, and the signer guarantees it never signs with
// a deactivated or expired key. Pruning expired keys is the job of
// the DeleteExpired cron, not the verifier.
func (v *VerifierLocal) Verify(ctx context.Context, kid, alg string, signingInput, signature []byte) error {
	if kid == "" {
		return ErrUnknownKID
	}

	key, err := v.repo.ReadByKeyID(kid)
	if errors.Is(err, entities.ErrSigningKeyNotFound) {
		return ErrUnknownKID
	}
	if err != nil {
		return fmt.Errorf("signing: read key by kid=%s: %w", kid, err)
	}

	if key.Algorithm != alg {
		return fmt.Errorf("%w: kid=%s, key alg=%s, token alg=%s",
			ErrAlgorithmMismatch, kid, key.Algorithm, alg)
	}

	switch alg {
	case entities.SigningAlgRS256:
		return verifyRS256(key.PublicKey, signingInput, signature, kid)
	default:
		return fmt.Errorf("%w: %q", ErrUnsupportedAlgorithm, alg)
	}
}

// verifyRS256 performs the cryptographic verification for RS256:
// PKCS#1 v1.5 over SHA-256.
func verifyRS256(publicKeyPEM string, signingInput, signature []byte, kid string) error {
	publicKey, err := ParseRSAPublicKeyFromPEM(publicKeyPEM)
	if err != nil {
		return fmt.Errorf("signing: parse public key for kid=%s: %w", kid, err)
	}

	hashed := sha256.Sum256(signingInput)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], signature); err != nil {
		// Don't wrap the underlying rsa error — the caller doesn't
		// need to distinguish between (e.g.) "signature too short" and
		// "signature mathematically invalid". Both are forged tokens.
		return ErrInvalidSignature
	}
	return nil
}
