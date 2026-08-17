package signing

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"auth/data/entities"
	"auth/data/repositories"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// kmsSignClient is the subset of the KMS client surface SignerKms uses.
// Defined as an interface so tests can inject a fake without depending
// on the AWS SDK's internals.
type kmsSignClient interface {
	Sign(ctx context.Context, params *kms.SignInput, optFns ...func(*kms.Options)) (*kms.SignOutput, error)
}

// SignerKms signs JWTs by delegating to AWS KMS via the kms:Sign API.
// The private key never leaves the HSM.
//
// The kid is resolved once (lazily, on first ActiveKeyID call) from the
// oauth_signing_keys table — auth-service's startup seeder
// (SeedKMSSigningKey) is responsible for ensuring a row exists with
// backend='kms' before the first sign. The cached kid persists for
// the lifetime of the process; rotation requires a redeploy.
//
// The current implementation hardcodes RS256 because we provision
// RSA_2048 KMS keys for the staging rollout. When ES256 keys are
// introduced, the algorithm choice will move to a per-row read or a
// configured value.
type SignerKms struct {
	client kmsSignClient
	alias  string
	repo   repositories.OAuthSigningKeysInterface
	alg    string

	// Cached kid — populated on first ActiveKeyID call, immutable after.
	mu        sync.RWMutex
	cachedKid string
}

// NewSignerKms returns a Signer that signs via AWS KMS.
//
// alias is the KMS alias (e.g. "alias/anahita-oauth-signing-key-staging").
// The AWS SDK resolves credentials and region from the standard env-var
// chain (AWS_REGION / AWS_DEFAULT_REGION, AWS_ACCESS_KEY_ID, etc.) plus
// ~/.aws/credentials and IRSA pod identity.
func NewSignerKms(ctx context.Context, alias string, repo repositories.OAuthSigningKeysInterface) (*SignerKms, error) {
	if alias == "" {
		return nil, errors.New("signing: kms key alias required")
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("signing: load aws config: %w", err)
	}

	return &SignerKms{
		client: kms.NewFromConfig(cfg),
		alias:  alias,
		repo:   repo,
		alg:    entities.SigningAlgRS256,
	}, nil
}

// Algorithm returns the JWA algorithm name this signer produces.
func (s *SignerKms) Algorithm() string {
	return s.alg
}

// ActiveKeyID returns the kid of the active KMS-backed signing key.
//
// First call: reads the active row from oauth_signing_keys and caches
// the kid + validates that the row's algorithm matches the signer's
// expected algorithm.
//
// Subsequent calls: return the cached kid without a DB read. The
// cache lives for the process lifetime — rotation goes through a
// redeploy, not at runtime.
//
// Returns ErrNoActiveSigningKey if no row exists (SeedKMSSigningKey
// didn't run, or the seed failed). Returns ErrAlgorithmMismatch if
// the row's algorithm doesn't match what the signer was constructed
// for — defends against the case where (e.g.) a local RS256 row is
// somehow active in KMS mode.
func (s *SignerKms) ActiveKeyID(ctx context.Context) (string, error) {
	s.mu.RLock()
	if s.cachedKid != "" {
		kid := s.cachedKid
		s.mu.RUnlock()
		return kid, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cachedKid != "" {
		return s.cachedKid, nil
	}

	key, err := s.repo.ReadActiveSigningKey()
	if errors.Is(err, entities.ErrSigningKeyNotFound) {
		return "", ErrNoActiveSigningKey
	}
	if err != nil {
		return "", fmt.Errorf("signing: read active key: %w", err)
	}
	if key.Algorithm != s.alg {
		return "", fmt.Errorf("%w: signer alg=%s, active key alg=%s, kid=%s",
			ErrAlgorithmMismatch, s.alg, key.Algorithm, key.KeyID)
	}

	s.cachedKid = key.KeyID
	return s.cachedKid, nil
}

// Sign delegates the cryptographic operation to AWS KMS. The signer
// passes the raw JWT signing input to KMS; KMS hashes (SHA-256) and
// signs (RSASSA-PKCS1-v1_5) in a single round-trip.
//
// AWS SDK v2 retries throttling and transient errors automatically
// with exponential backoff, so callers don't need their own retry
// loop. The default 1200 req/sec asymmetric Sign quota per region is
// well above auth-service's expected token-mint rate; if hit, request
// a quota increase via AWS Support.
func (s *SignerKms) Sign(ctx context.Context, signingInput []byte) ([]byte, error) {
	out, err := s.client.Sign(ctx, &kms.SignInput{
		KeyId:            aws.String(s.alias),
		Message:          signingInput,
		MessageType:      types.MessageTypeRaw,
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	})
	if err != nil {
		return nil, fmt.Errorf("signing: kms sign: %w", err)
	}
	return out.Signature, nil
}
