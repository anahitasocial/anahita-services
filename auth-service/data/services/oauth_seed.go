package services

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	libs_constants "anahita-libs/constants"
	"auth/data/entities"
	"auth/data/repositories"
	"auth/internal/signing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// SigningKeyValidity is the validity window for newly seeded signing
// keys. Kept as a package-level default; main.go passes
// cfg.GetOAuthKeyLifetime() at the call site, which overrides this
// when a different env value is set.
const SigningKeyValidity = 90 * 24 * time.Hour

// ============================================================================
// SeedSigningKey — bootstrap the active OAuth signing key (LOCAL mode)
// ============================================================================

// SeedSigningKey ensures at least one active signing key exists. If
// one already exists, this is a no-op. If the environment provides a
// PEM file path, the key is imported from there; otherwise a fresh
// keypair is generated and persisted.
//
// Local-mode only. main.go skips this call when OAUTH_SIGNER_BACKEND
// is "kms" — in that case SeedKMSSigningKey runs instead.
//
// Call this at service startup after Validate has approved the config.
func SeedSigningKey(repos *repositories.Repositories, privateKeyPath string, keyLifetime time.Duration) error {
	if _, err := repos.OAuthSigningKeys.ReadActiveSigningKey(); err == nil {
		return nil
	} else if !isNotFoundErr(err) {
		return fmt.Errorf("check for existing signing key: %w", err)
	}

	var privatePEM string
	var priv *rsa.PrivateKey

	if privateKeyPath != "" {
		data, err := os.ReadFile(privateKeyPath)
		if err != nil {
			return fmt.Errorf("read private key file: %w", err)
		}

		privatePEM = string(data)
		priv, err = signing.ParseRSAPrivateKeyFromPEM(privatePEM)
		if err != nil {
			return fmt.Errorf("parse private key from file: %w", err)
		}
	} else {
		var err error
		priv, err = signing.GenerateRSAKeyPair(2048)
		if err != nil {
			return fmt.Errorf("generate initial signing key: %w", err)
		}
		privatePEM = signing.MarshalRSAPrivateKeyToPEM(priv)
	}

	publicPEM, err := signing.MarshalRSAPublicKeyToPEM(&priv.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal public key: %w", err)
	}

	kid, err := signing.GenerateKeyID()
	if err != nil {
		return fmt.Errorf("generate kid: %w", err)
	}

	key := entities.NewOAuthSigningKey()
	key.KeyID = kid
	key.Algorithm = entities.SigningAlgRS256
	key.PrivateKey = privatePEM
	key.PublicKey = publicPEM
	key.ExpiresAt = time.Now().Add(keyLifetime)

	if _, err := repos.OAuthSigningKeys.Add(key); err != nil {
		return fmt.Errorf("persist initial signing key: %w", err)
	}

	log.Printf("oauth: seeded initial signing key kid=%s alg=%s", kid, key.Algorithm)
	return nil
}

// ============================================================================
// SeedKMSSigningKey — bootstrap the KMS-backed OAuth signing key (KMS mode)
// ============================================================================

// SeedKMSSigningKey ensures an oauth_signing_keys row exists for the
// configured KMS-backed signing key. The KMS key itself is provisioned
// out of band (in AWS); this function creates the database
// representation that:
//
//  1. Lets SignerKms resolve a kid via the standard repository path
//  2. Lets the JWKS endpoint publish the public key for verifiers
//  3. Lets VerifierLocal verify in-flight tokens without a KMS call
//
// Idempotency is keyed on the signer's own predicate
// (ReadActiveSigningKey), not on kid existence. Checking by kid alone
// is wrong: a row can exist for the kid while being inactive or past
// expires_at, in which case the signer fails with "read active key:
// sql: no rows in result set" and a kid-based seeder skips instead of
// healing. The upsert below therefore reactivates and extends an
// existing row rather than assuming absence.
//
// Uses raw SQL because OAuthSigningKeys.Add doesn't support NULL
// private_key, which the schema's CHECK constraint requires for
// kms-backend rows. The alternative — adding an AddKMSKey method to
// the repo — is a follow-up cleanup; the raw SQL is contained to this
// one call site.
func SeedKMSSigningKey(ctx context.Context, conn *sql.DB, repos *repositories.Repositories, alias string) error {
	// Match the signer exactly. If the signer can resolve an active
	// key, there is nothing to do; if it can't, we must (re)seed even
	// when a row for this kid is already present.
	if active, err := repos.OAuthSigningKeys.ReadActiveSigningKey(); err == nil && active != nil {
		log.Printf("oauth: active signing key present, kid=%s alg=%s", active.KeyID, active.Algorithm)
		return nil
	} else if err != nil && !isNotFoundErr(err) {
		return fmt.Errorf("check for existing signing key: %w", err)
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}
	client := kms.NewFromConfig(cfg)

	descOut, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: aws.String(alias),
	})
	if err != nil {
		return fmt.Errorf("describe kms key: %w", err)
	}
	if descOut.KeyMetadata == nil {
		return fmt.Errorf("describe kms key: empty metadata for alias %s", alias)
	}

	md := descOut.KeyMetadata

	// Sanity: confirm the KMS key is actually configured for signing.
	// Catches "wrong key" misconfigurations before we try to use it.
	if md.KeyUsage != types.KeyUsageTypeSignVerify {
		return fmt.Errorf("kms key %s has KeyUsage=%s, expected SIGN_VERIFY",
			alias, md.KeyUsage)
	}

	var alg string
	switch md.KeySpec {
	case types.KeySpecRsa2048:
		alg = entities.SigningAlgRS256
	default:
		// Add cases for ECC_NIST_P256 (ES256) etc. when those keys are
		// first provisioned. SignerKms will also need a corresponding
		// SigningAlgorithm update.
		return fmt.Errorf("unsupported KMS key spec: %s", md.KeySpec)
	}

	// Use the KMS key UUID as the kid. This makes "which KMS key
	// signed this token?" answerable by looking the kid up in the AWS
	// console. Naturally rotates when the KMS key is replaced.
	kid := aws.ToString(md.KeyId)

	pubOut, err := client.GetPublicKey(ctx, &kms.GetPublicKeyInput{
		KeyId: aws.String(alias),
	})
	if err != nil {
		return fmt.Errorf("get kms public key: %w", err)
	}

	// KMS returns the public key as DER (X.509 SubjectPublicKeyInfo).
	// Wrap in PEM so the JWKS endpoint and VerifierLocal — both of
	// which expect PEM — work without special-casing the source.
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubOut.PublicKey,
	})

	// expires_at semantics differ for KMS: the AWS-side key has its
	// own lifecycle managed in IAM/KMS, and rotation is driven by
	// replacing the key (which yields a new kid, hence a new row).
	// The DB column exists for schema compatibility and to satisfy
	// ReadActiveSigningKey's freshness filter — VerifierLocal doesn't
	// read it. Because the AWS key does not expire on this schedule,
	// the window is extended on every restart by the upsert below,
	// which is what keeps a long-lived staging deployment from
	// silently aging out of a valid signing key.
	expiresAt := time.Now().Add(SigningKeyValidity)

	// Upsert keyed on key_id, which carries a UNIQUE index in the
	// schema — that index is what ON DUPLICATE KEY UPDATE fires on.
	//
	// The updated values are passed a second time rather than read back
	// with VALUES(col): that function is deprecated as of MySQL 8.0.20,
	// and the replacement row-alias syntax (`AS new`) is a parse error
	// on 8.0.19 and earlier. Repeating the parameters works on every
	// version and costs nothing here.
	_, err = conn.ExecContext(ctx, `
		INSERT INTO oauth_signing_keys
			(key_id, algorithm, public_key, private_key, backend, active, expires_at)
		VALUES (?, ?, ?, NULL, 'kms', TRUE, ?)
		ON DUPLICATE KEY UPDATE
			algorithm  = ?,
			public_key = ?,
			backend    = 'kms',
			active     = TRUE,
			expires_at = ?
	`, kid, alg, string(publicPEM), expiresAt, alg, string(publicPEM), expiresAt)
	if err != nil {
		return fmt.Errorf("persist kms signing key: %w", err)
	}

	log.Printf("oauth: seeded KMS signing key kid=%s alg=%s alias=%s", kid, alg, alias)
	return nil
}

// ============================================================================
// SeedDefaultClients — bootstrap first-party OAuth clients
// ============================================================================

// seedClientsDocument is the on-disk shape of oauth_seed_clients.json.
type seedClientsDocument struct {
	Clients []seedClient `json:"clients"`
}

// seedClient mirrors entities.OAuthClient for the seed file. Field
// tags use snake_case to match the file's JSON convention.
//
// Public clients (Confidential=false) require no secret. Confidential
// clients seeded via this file would need a separate secret-injection
// strategy and aren't supported here — the seed file is meant for
// public first-party apps only.
type seedClient struct {
	ClientID     string   `json:"client_id"`
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
	GrantTypes   []string `json:"grant_types"`
	Scopes       []string `json:"scopes"`
	TokenExpiry  int      `json:"token_expiry"`
	Confidential bool     `json:"confidential"`
	SkipConsent  bool     `json:"skip_consent"`
	FirstParty   bool     `json:"first_party"`
	Active       bool     `json:"active"`
}

// SeedDefaultClients reads a JSON file of OAuth client definitions
// and inserts any that aren't already registered.
//
// Idempotent: a client_id that already exists in the database is left
// untouched. Updates to redirect URIs, scopes, etc. of an existing
// client must go through the /clients admin endpoint — the seed file
// is bootstrap-only, not a source of truth.
//
// The skip is deliberate and it is a footgun: editing scopes in the
// seed file and restarting does NOTHING to an already-seeded client,
// and the only signal is one easily-missed "already exists, skipping"
// log line. It is the right default because it guarantees a redeploy
// never silently reverts an operator's edit made through /clients.
//
// To apply a changed shipped scope list you must delete the
// oauth_clients row and restart — which cascades to that client's
// refresh tokens and authorizations and logs every one of its users
// out. On a live site, PATCH /clients/:id instead.
//
// Confidential clients in the seed file are rejected with an error.
// Confidential clients need secrets that can't be safely committed to
// a repo; register them via the admin endpoint instead.
func SeedDefaultClients(repos *repositories.Repositories, path string) error {
	if path == "" {
		log.Println("oauth: seed clients path empty, skipping")
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// Missing file is non-fatal — operators may run without seed
		// file in production and manage clients via the admin endpoint
		// only. Log and move on.
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("oauth: seed clients file not found at %s, skipping", path)
			return nil
		}
		return fmt.Errorf("read seed clients file: %w", err)
	}

	var doc seedClientsDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse seed clients file: %w", err)
	}

	for _, sc := range doc.Clients {
		if err := seedSingleClient(repos, sc); err != nil {
			return fmt.Errorf("seed client %q: %w", sc.ClientID, err)
		}
	}

	return nil
}

// seedSingleClient inserts a single client if it doesn't already exist.
func seedSingleClient(repos *repositories.Repositories, sc seedClient) error {
	if sc.ClientID == "" {
		return errors.New("client_id is required")
	}
	if sc.Confidential {
		return errors.New("confidential clients cannot be seeded from file; use the admin endpoint")
	}

	// A scope the catalog doesn't know is unusable — authorize would
	// reject the whole request with invalid_scope. Fail at boot, where a
	// typo in the shipped JSON is one log line away from the person who
	// made it, rather than seeding a client nobody can log in with.
	for _, scope := range sc.Scopes {
		if !libs_constants.OAuthScopeExists(scope) {
			return fmt.Errorf("unknown scope %q", scope)
		}
	}

	if existing, err := repos.OAuthClients.ReadByClientID(sc.ClientID); err == nil && existing != nil {
		log.Printf("oauth: seed client %q already exists, skipping", sc.ClientID)
		return nil
	}

	client := entities.NewOAuthClient()
	client.ClientID = sc.ClientID
	client.Name = sc.Name
	client.RedirectURIs = sc.RedirectURIs
	client.GrantTypes = sc.GrantTypes
	client.Scopes = entities.StringSlice(sc.Scopes)
	client.TokenExpiry = sc.TokenExpiry
	client.Confidential = sc.Confidential
	client.SkipConsent = sc.SkipConsent
	client.FirstParty = sc.FirstParty
	client.Active = sc.Active
	client.CreatedAt = time.Now()

	if err := client.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if _, err := repos.OAuthClients.Add(client); err != nil {
		return fmt.Errorf("persist: %w", err)
	}

	log.Printf("oauth: seeded client %q (%s)", sc.ClientID, sc.Name)
	return nil
}

// ============================================================================
// Helpers
// ============================================================================

// isNotFoundErr reports whether err signals "no such row" from the
// signing-keys repository. It accepts:
//
//   - entities.ErrSigningKeyNotFound: the domain sentinel
//   - sql.ErrNoRows: the database/sql sentinel that the repo currently
//     leaks through directly for some methods
//   - any error whose message contains "no rows in result set": fallback
//     for cases where the repo wraps with %v or %s instead of %w,
//     breaking errors.Is. This is defensive — once the repo is fixed
//     to translate sql.ErrNoRows → ErrSigningKeyNotFound at the
//     boundary, the string check can be deleted.
func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, entities.ErrSigningKeyNotFound) {
		return true
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	return strings.Contains(err.Error(), "no rows in result set")
}
