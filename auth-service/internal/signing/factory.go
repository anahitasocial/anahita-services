package signing

import (
	"context"
	"fmt"

	"auth/data/entities"
	"auth/data/repositories"

	"anahita-libs/constants"
)

// FactoryConfig is the subset of auth-service Config that the factory
// reads. Defining this as an interface lets tests construct signers
// without instantiating a full Config and avoids a circular import
// between the signing and config packages.
type FactoryConfig interface {
	GetOAuthSignerBackend() string
	GetOAuthKMSKeyAlias() string
}

// NewSigner returns a Signer matching the configured backend.
//
// Local mode signs using a private key stored in oauth_signing_keys,
// pinned to RS256. KMS mode delegates to AWS KMS via the kms:Sign API
// and uses the same RS256 algorithm (RSA_2048 KMS keys).
//
// ctx is used for AWS SDK config loading in KMS mode. Pass
// context.Background() at startup; the SDK uses ctx primarily for
// cancellation during credential resolution.
func NewSigner(ctx context.Context, cfg FactoryConfig, repo repositories.OAuthSigningKeysInterface) (Signer, error) {
	switch cfg.GetOAuthSignerBackend() {
	case constants.OAuthSignerBackendLocal:
		return NewSignerLocal(repo, entities.SigningAlgRS256)
	case constants.OAuthSignerBackendKMS:
		return NewSignerKms(ctx, cfg.GetOAuthKMSKeyAlias(), repo)
	default:
		return nil, fmt.Errorf("signing: unknown signer backend %q", cfg.GetOAuthSignerBackend())
	}
}

// NewVerifier returns a Verifier matching the configured backend.
//
// Both local and KMS backends use VerifierLocal because verification
// reads from the oauth_signing_keys table in either case — KMS-backed
// keys persist their public key in the same row at provisioning time.
// This is intentional: it means downstream microservices can verify
// tokens with database access alone, no KMS IAM permissions required.
//
// The factory still switches on backend so an unknown backend value
// fails at startup with the same error shape NewSigner produces,
// keeping diagnostics consistent.
func NewVerifier(cfg FactoryConfig, repo repositories.OAuthSigningKeysInterface) (Verifier, error) {
	switch cfg.GetOAuthSignerBackend() {
	case constants.OAuthSignerBackendLocal, constants.OAuthSignerBackendKMS:
		return NewVerifierLocal(repo), nil
	default:
		return nil, fmt.Errorf("signing: unknown signer backend %q", cfg.GetOAuthSignerBackend())
	}
}
