package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"auth/cmd/api/handlers"
	"auth/cmd/api/notifiers"
	"auth/cmd/api/permissions"
	"auth/config"
	"auth/data/repositories"
	"auth/data/services"
	"auth/internal/signing"

	"anahita-libs/constants"
	"anahita-libs/crypt"
	"anahita-libs/data"
	"anahita-libs/event"
	"anahita-libs/middleware"
	"anahita-libs/sessions"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"
)

const webPort = "80"

// Compile-time assertion that auth-service's Config satisfies the
// anahita-libs middleware contract. If a method is added to
// ConfigInterface and Config doesn't implement it, this line catches
// the gap at build time instead of as a confusing runtime panic.
var _ middleware.ConfigInterface = (*config.Config)(nil)

// Compile-time assertion that Config also satisfies the signing
// factory's required interface (GetOAuthSignerBackend,
// GetOAuthKMSKeyAlias). Same fail-fast principle.
var _ signing.FactoryConfig = (*config.Config)(nil)

// Compile-time assertion that Config satisfies the crypt factory's
// required interface (GetCryptBackend, GetCryptLocalKey,
// GetAwsKmsKeyId). Same fail-fast principle.
var _ crypt.ConfigInterface = (*config.Config)(nil)

func main() {
	// 1. Configuration.
	cfg := config.NewConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	// 2. Echo bootstrap.
	e := echo.New()
	e.Use(echo_middleware.RequestLogger())
	e.Use(echo_middleware.Recover())
	e.Use(echo_middleware.BodyLimit(constants.RequestBodyMaxSizeLimit))
	e.Validator = validators.NewRequestValidator(validator.New())
	if cfg.IsDev() {
		e.Debug = true
	}

	// 3. Database.
	conn := data.ConnectToDB(cfg.GetDSN())
	if conn == nil {
		log.Panic("can't connect to PostgreSQL!")
	}

	// 4. Single instances of shared dependencies. Each is constructed
	// once and shared across permissions / notifiers / handlers env
	// structs. Constructing them multiple times wastes statement caches
	// and makes lifecycle reasoning harder.
	repos := repositories.NewRepositories(conn)
	srv := services.NewServices(cfg)
	session := sessions.NewSession(cfg)
	cryptEnc := &crypt.Encryption{}

	// Secrets-at-rest encryption (TOTP secrets). The factory selects
	// local or AWS KMS from CRYPT_BACKEND, defaulting to local so a
	// deployment without AWS works. Built once here rather than per
	// request so a bad key fails at boot instead of mid-enrolment.
	secretCrypt, err := crypt.NewCrypt(cfg)
	if err != nil {
		log.Fatalf("failed to construct crypt: %v", err)
	}

	// 5. Event publishing infrastructure.
	amqpConn, err := event.ConnectToAMQP(cfg)
	if err != nil {
		log.Panic(err)
	}

	publisher, err := event.NewEventPublisher(amqpConn)
	if err != nil {
		log.Panic(err)
	}

	// 6. OAuth signer + verifier.
	//
	// Signing is delegated to the Signer interface so OAuthService can
	// switch between a local DB-backed key (dev/CI) and AWS KMS (cloud)
	// without code changes. The factory selects the implementation
	// based on OAUTH_SIGNER_BACKEND in config; an unknown or
	// unsupported backend fails loudly here at startup.
	//
	// The verifier is database-backed in both modes — KMS-backed keys
	// persist their public key in the same oauth_signing_keys row at
	// provisioning time, so verification stays a database read with
	// no KMS IAM required. This is what lets downstream microservices
	// verify tokens with DB access alone.
	//
	// startupCtx is shared across signer factory and KMS seeder. The
	// AWS SDK uses it for credential resolution and KMS API calls
	// during one-shot startup; Background is fine because there's no
	// request to inherit cancellation from at this point.
	startupCtx := context.Background()

	signer, err := signing.NewSigner(startupCtx, cfg, repos.OAuthSigningKeys)
	if err != nil {
		log.Fatalf("failed to construct OAuth signer: %v", err)
	}

	verifier, err := signing.NewVerifier(cfg, repos.OAuthSigningKeys)
	if err != nil {
		log.Fatalf("failed to construct OAuth verifier: %v", err)
	}

	// 7. OAuth service. Owns scope validation, token issuance/refresh/
	// revocation, JWKS document construction. Cryptographic operations
	// go through signer / verifier above.
	oauth := services.NewOAuthService(repos, srv, signer, verifier, cfg.GetOAuthIssuer())

	// 8. OAuth one-time startup tasks.
	//
	// The signing-key seeder branches on backend:
	//
	// Local mode → SeedSigningKey ensures an active DB-backed key
	// exists. Imports from OAUTH_PRIVATE_KEY_PATH if set, otherwise
	// generates a fresh keypair.
	//
	// KMS mode → SeedKMSSigningKey ensures the oauth_signing_keys row
	// exists for the configured KMS key. Calls KMS DescribeKey to
	// validate the alias resolves to an RSA_2048 SIGN_VERIFY key,
	// then GetPublicKey to fetch the public material, then inserts a
	// row with backend='kms' and private_key=NULL. Idempotent: skips
	// if a row for the kid already exists. The KMS key itself is
	// provisioned in AWS console / Terraform — main.go does not
	// create KMS keys.
	//
	// The default branch is unreachable in practice (config.Validate
	// rejects unknown backend values) but exists as a defense in case
	// validation is ever bypassed.
	switch cfg.GetOAuthSignerBackend() {
	case constants.OAuthSignerBackendLocal:
		if err := services.SeedSigningKey(repos, cfg.GetOAuthPrivateKeyPath(), cfg.GetOAuthKeyLifetime()); err != nil {
			log.Fatalf("failed to seed local OAuth signing key: %v", err)
		}

	case constants.OAuthSignerBackendKMS:
		if err := services.SeedKMSSigningKey(startupCtx, conn, repos, cfg.GetOAuthKMSKeyAlias()); err != nil {
			log.Fatalf("failed to seed KMS OAuth signing key: %v", err)
		}

	default:
		log.Fatalf("unknown OAUTH_SIGNER_BACKEND: %q", cfg.GetOAuthSignerBackend())
	}

	// SeedDefaultClients runs in both modes — clients are configuration
	// data, unrelated to which backend signs tokens. Idempotent: clients
	// already registered in the database are left untouched.
	if err := services.SeedDefaultClients(repos, cfg.GetOAuthSeedClientPath()); err != nil {
		log.Fatalf("failed to seed default OAuth clients: %v", err)
	}

	// TODO: LoadScopes reads OAUTH_SCOPES_PATH for runtime scope
	// metadata (descriptions, etc.). Skipped for now — scope names
	// live in anahita-libs/constants and are sufficient until consent
	// UI needs the descriptions.
	// if err := services.LoadScopes(cfg.GetOAuthScopesPath()); err != nil {
	// 	log.Printf("failed to load scope definitions: %v", err)
	// }

	// 9. Signer sanity check. Confirms the configured signer can
	// actually resolve an active key after seeding completed. Catches
	// misconfigurations where seeding succeeded but the signer's
	// algorithm filter rejects the seeded key (e.g. signer pinned to
	// ES256, key in DB is RS256), or where KMS mode booted with no
	// pre-provisioned key.
	//
	// Logging the backend alongside kid/alg makes staging logs make it
	// obvious which signing path is live without grepping config.
	if kid, err := signer.ActiveKeyID(startupCtx); err != nil {
		log.Fatalf("oauth: signer cannot resolve active key: %v", err)
	} else {
		log.Printf("oauth: signing with kid=%s alg=%s backend=%s",
			kid, signer.Algorithm(), cfg.GetOAuthSignerBackend())
	}

	// 10. JWKS client. One per process. Used by OAuth middleware to
	// verify JWTs locally on every protected route. Auth-service is
	// its own JWKS publisher and consumer — same client construction
	// as any other microservice.
	jwks := middleware.NewJWKSClient(cfg.GetOAuthJWKSURL())

	// 11. Pre-built middleware functions for use in routes.
	authorize := middleware.Authorize(cfg, jwks)
	setViewer := middleware.SetViewer(cfg, jwks)

	// 12. Permissions, notifiers, handlers.
	pEnv := &permissions.Env{
		Config: cfg,
		Repos:  repos,
	}
	perms := permissions.NewPermissions(pEnv)

	nEnv := &notifiers.Env{
		Config:    cfg,
		Repos:     repos,
		Srv:       srv,
		Publisher: publisher,
	}
	notif := notifiers.NewNotifiers(nEnv)

	hEnv := &handlers.Env{
		Config:      cfg,
		Crypt:       cryptEnc,
		SecretCrypt: secretCrypt,
		Notif:       notif,
		OAuth:       oauth,
		Perms:       perms,
		Repos:       repos,
		Session:     session,
		Srv:         srv,
		Publisher:   publisher,
	}
	h := handlers.NewHandlers(hEnv)

	// 13. Routes.
	// authorize and setViewer are passed through so route groups can
	// apply per-group middleware. setViewer is NOT applied globally —
	// public OAuth/well-known endpoints should not run JWT validation.
	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	NewRoutes(h, authorize, setViewer).Init(e)

	// 14. Startup banner. Logs the configured OAuth issuer/JWKS URL
	// for sanity-checking — issuer mismatch is a common misconfig that
	// looks like "auth is broken" without an obvious cause.
	log.Printf("oauth: issuer=%s jwks=%s", cfg.GetOAuthIssuer(), cfg.GetOAuthJWKSURL())
	log.Printf("auth-service listening on :%s", webPort)

	if err := e.Start(fmt.Sprintf(":%v", webPort)); err != nil {
		e.Logger.Fatal(err)
	}
}
