package config

import (
	"anahita-libs/constants"
	"anahita-libs/crypt"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ErrMissingDSN                 = "DSN is missing"
	ErrMissingSecureCookieKey     = "Secure cookie key is missing"
	ErrWrongSizeSecureCookieKey   = "Secure cookie key must be 32 bytes long"
	ErrMissingRedisAddr           = "Redis address is missing"
	ErrMissingDomainName          = "Domain name is missing"
	ErrMissingDomainNameFormat    = "Domain name should not contain 'http' or 'www'"
	ErrMissingSystemEmail         = "system email is missing"
	ErrMissingSupportEmail        = "support email is missing"
	ErrMissingAwsKmsKeyId         = "aws KMS Key id is missing"
	ErrMissingCryptLocalKey       = "crypt local key is missing"
	ErrWrongSizeCryptLocalKey     = "crypt local key must be 32 bytes long"
	ErrMissingMailServiceAddr     = "mail service address is missing"
	ErrMissingFileAddr            = "File gRPC service address is missing"
	ErrMissingAmqpAddr            = "AMQP address is missing"
	ErrMissingOAuthIssuer         = "OAuth issuer is missing"
	ErrMissingOAuthJWKSURL        = "OAuth JWKS URL is missing"
	ErrMissingOAuthSeedClientPath = "OAuth seed client path is missing"
	ErrInvalidOAuthKeyLifetime    = "OAuth key lifetime must be positive"
	ErrMissingOAuthKMSKeyAlias    = "OAuth KMS key alias is missing"
)

const (
	defaultOAuthSeedClientPath = "defaults/oauth_seed_clients.json"
	// defaultOAuthKeyLifetimeDays is the default key rotation interval.
	// 90 days is the common operational sweet spot — long enough to
	// avoid frequent rotation churn, short enough to limit blast
	// radius if a key is compromised. Override via OAUTH_KEY_LIFETIME_DAYS.
	defaultOAuthKeyLifetimeDays = 90
)

type Config struct {
	Env              string
	MaxDBConnections int
	DSN              string
	SecureCookieKey  string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	DomainName       string
	SystemEmail      string
	// SiteName is what this installation calls itself — the "Banu" in
	// "Your Banu password was changed".
	//
	// It appears in two places, both of which are trust signals rather
	// than decoration: the From display name a recipient sees in their
	// inbox list, and the text of the notification itself. A security
	// notice that knows which installation it is about is harder to
	// mistake for phishing, and phishing that does not know the name is
	// easier to spot.
	//
	// Config rather than a literal because another installation is
	// Banu.io rather than Anahita. Optional — GetSiteName falls back to
	// DomainName, so a deployment that never sets it still reads
	// sensibly.
	SiteName string
	// TotpIssuer is the label an authenticator app shows beside a
	// person's 2FA code — the "Anahita" in "Anahita (you@example.com)".
	//
	// Optional. Unset, GetTotpIssuer derives one from DomainName, which
	// is what makes this a config value rather than the hardcoded
	// "anahita.io" it replaces: that string was wrong for every
	// deployment but ours, and it is baked into the QR code, so nobody
	// could correct it after the fact without re-enrolling.
	//
	// Cosmetic, not a security control. The issuer travels in the
	// otpauth:// URI and is displayed; it is not mixed into the code.
	// Changing it does not invalidate existing enrolments — already-
	// paired authenticators keep the label they were given, so a change
	// shows up only for people who enrol after it.
	TotpIssuer      string
	SupportEmail    string
	MailServiceAddr string
	FileServiceAddr string
	AmqpAddr        string

	// Crypt — symmetric encryption of secrets at rest (currently TOTP
	// secrets).
	//
	// CryptBackend selects the implementation: "local" encrypts with
	// CryptLocalKey and has no cloud dependency, "kms" delegates to AWS
	// KMS using AwsKmsKeyId. Defaults to "local" so a deployment
	// without AWS works out of the box. Normalized to lowercase by
	// NewConfig so "KMS" or "Local" are accepted.
	//
	// The two backends produce incompatible ciphertexts. Switching an
	// existing deployment leaves stored secrets undecryptable — people
	// with 2FA enabled have to re-enrol.
	CryptBackend string
	// CryptLocalKey is the 32-byte AES-256 key used when CryptBackend
	// is "local". Distinct from SecureCookieKey on purpose: rotating a
	// session key should not silently destroy every stored TOTP secret.
	CryptLocalKey string
	// AwsKmsKeyId is the KMS key used when CryptBackend is "kms".
	AwsKmsKeyId string

	// OAuth
	OAuthIssuer         string
	OAuthJWKSURL        string
	OAuthSeedClientPath string
	// OAuthPrivateKeyPath is an optional dev-mode shortcut: if set,
	// SeedSigningKey reads the PEM at this path instead of generating
	// a fresh key. In production with KMS, leave empty.
	OAuthPrivateKeyPath string
	// OAuthKeyLifetime is how long a freshly seeded signing key stays
	// valid before rotation. Old keys remain in JWKS until expiry so
	// outstanding tokens can still be verified.
	OAuthKeyLifetime time.Duration
	// OAuthSignerBackend determines how auth-service signs JWTs.
	// "local" uses a locally generated key, "kms" uses AWS KMS.
	// Normalized to lowercase by NewConfig so operator-supplied values
	// like "KMS" or "Local" are accepted.
	OAuthSignerBackend string
	// OAuthKMSKeyAlias is the alias of the AWS KMS key used for signing
	// when OAuthSignerBackend is "kms". This should be the alias,
	// not the full ARN, e.g. "alias/auth-service-signing-key". The KMS key must
	// allow the "kms:Sign" action for the IAM role of auth-service.
	OAuthKMSKeyAlias string
}

func NewConfig() *Config {
	config := &Config{
		Env:              getEnv("ENV", constants.ConfigEnvDev),
		DSN:              os.Getenv("DSN"),
		MaxDBConnections: getIntEnv("MAX_DB_CONNECTIONS", constants.ConfigDefaultDBConnections),
		SecureCookieKey:  os.Getenv("SECURE_COOKIE_KEY"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		RedisDB:          getIntEnv("REDIS_DB", 0),
		DomainName:       os.Getenv("DOMAIN_NAME"),
		SystemEmail:      os.Getenv("SYSTEM_EMAIL"),
		SiteName:         os.Getenv("SITE_NAME"),
		TotpIssuer:       os.Getenv("TOTP_ISSUER"),
		SupportEmail:     os.Getenv("SUPPORT_EMAIL"),
		CryptBackend:     getEnv("CRYPT_BACKEND", constants.CryptBackendLocal),
		CryptLocalKey:    os.Getenv("CRYPT_LOCAL_KEY"),
		AwsKmsKeyId:      os.Getenv("AWS_KMS_KEY_ID"),
		MailServiceAddr:  os.Getenv("MAIL_SERVICE_ADDR"),
		AmqpAddr:         os.Getenv("AMQP_ADDR"),
		FileServiceAddr:  os.Getenv("FILE_SERVICE_ADDR"),

		// OAuth
		OAuthIssuer:         os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:        os.Getenv("OAUTH_JWKS_URL"),
		OAuthSeedClientPath: getEnv("OAUTH_SEED_CLIENT_PATH", defaultOAuthSeedClientPath),
		OAuthPrivateKeyPath: os.Getenv("OAUTH_PRIVATE_KEY_PATH"),
		OAuthKeyLifetime:    parseKeyLifetime(),
		OAuthSignerBackend:  getEnv("OAUTH_SIGNER_BACKEND", constants.OAuthSignerBackendLocal),
		OAuthKMSKeyAlias:    os.Getenv("OAUTH_KMS_KEY_ALIAS"),
	}

	// Normalize backend selectors to lowercase so operator-supplied
	// values like "KMS" or "Local" match the constants. The constants
	// themselves are already lowercase; this just normalizes input.
	config.OAuthSignerBackend = strings.ToLower(config.OAuthSignerBackend)
	config.CryptBackend = strings.ToLower(config.CryptBackend)

	// If JWKS URL is unset, derive it from the issuer.
	if config.OAuthJWKSURL == "" && config.OAuthIssuer != "" {
		config.OAuthJWKSURL = strings.TrimRight(config.OAuthIssuer, "/") + "/.well-known/jwks.json"
	}

	return config
}

// parseKeyLifetime reads OAUTH_KEY_LIFETIME_DAYS from the environment
// and returns it as a duration. Defaults to 90 days. Returns 0 (which
// Validate will catch) if the env var is set but unparseable.
func parseKeyLifetime() time.Duration {
	raw := os.Getenv("OAUTH_KEY_LIFETIME_DAYS")
	if raw == "" {
		return time.Duration(defaultOAuthKeyLifetimeDays) * 24 * time.Hour
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return 0
	}
	return time.Duration(days) * 24 * time.Hour
}

// Validate checks that all required configuration settings are present and correct
func (config *Config) Validate() error {
	var missingFields []string

	if len(config.SecureCookieKey) == 0 {
		missingFields = append(missingFields, ErrMissingSecureCookieKey)
	}

	if len(config.SecureCookieKey) != 32 {
		missingFields = append(missingFields, ErrWrongSizeSecureCookieKey)
	}

	if config.DSN == "" {
		missingFields = append(missingFields, ErrMissingDSN)
	}

	// Redis is not optional. sessions.NewSession falls back to
	// cookie-backed sessions when this is empty, and that backend
	// stores session state in the client's cookie — there is nothing
	// server-side to delete. Session revocation silently becomes a
	// no-op, which means password reset stops invalidating an
	// attacker's IdP session while appearing to succeed.
	if config.RedisAddr == "" {
		missingFields = append(missingFields, ErrMissingRedisAddr)
	}

	if config.DomainName == "" {
		missingFields = append(missingFields, ErrMissingDomainName)
	} else if strings.Contains(config.DomainName, "http") || strings.Contains(config.DomainName, "www") {
		missingFields = append(missingFields, ErrMissingDomainNameFormat)
	}

	if config.SystemEmail == "" {
		missingFields = append(missingFields, ErrMissingSystemEmail)
	}

	if config.SupportEmail == "" {
		missingFields = append(missingFields, ErrMissingSupportEmail)
	}

	// Crypt backend. Only the selected backend's settings are required
	// — a deployment running "local" must not be forced to invent an
	// AWS KMS key id it will never use.
	switch config.CryptBackend {
	case constants.CryptBackendLocal:
		if config.CryptLocalKey == "" {
			missingFields = append(missingFields, ErrMissingCryptLocalKey)
		} else if len(config.CryptLocalKey) != crypt.LocalCryptKeySize {
			missingFields = append(missingFields, ErrWrongSizeCryptLocalKey)
		}
	case constants.CryptBackendKMS:
		if config.AwsKmsKeyId == "" {
			missingFields = append(missingFields, ErrMissingAwsKmsKeyId)
		}
	default:
		missingFields = append(missingFields, fmt.Sprintf(
			"CRYPT_BACKEND must be %q or %q (got %q)",
			constants.CryptBackendLocal,
			constants.CryptBackendKMS,
			config.CryptBackend,
		))
	}

	if config.MailServiceAddr == "" {
		missingFields = append(missingFields, ErrMissingMailServiceAddr)
	}

	if config.AmqpAddr == "" {
		missingFields = append(missingFields, ErrMissingAmqpAddr)
	}

	if config.OAuthIssuer == "" {
		missingFields = append(missingFields, ErrMissingOAuthIssuer)
	}

	if config.OAuthJWKSURL == "" {
		missingFields = append(missingFields, ErrMissingOAuthJWKSURL)
	}

	if config.OAuthSeedClientPath == "" {
		missingFields = append(missingFields, ErrMissingOAuthSeedClientPath)
	}

	if config.OAuthKeyLifetime <= 0 {
		missingFields = append(missingFields, ErrInvalidOAuthKeyLifetime)
	}

	// Validate signer backend is one of the supported values. Catches
	// typos like "loca" or "kmss" at startup instead of letting them
	// surface as confusing factory errors later. Includes the offending
	// value in the message so the operator doesn't have to guess what
	// they typed.
	switch config.OAuthSignerBackend {
	case constants.OAuthSignerBackendLocal, constants.OAuthSignerBackendKMS:
		// ok
	default:
		missingFields = append(missingFields, fmt.Sprintf(
			"OAUTH_SIGNER_BACKEND must be %q or %q (got %q)",
			constants.OAuthSignerBackendLocal,
			constants.OAuthSignerBackendKMS,
			config.OAuthSignerBackend,
		))
	}

	if config.OAuthSignerBackend == constants.OAuthSignerBackendKMS && config.OAuthKMSKeyAlias == "" {
		missingFields = append(missingFields, ErrMissingOAuthKMSKeyAlias)
	}

	if len(missingFields) > 0 {
		return errors.New("missing fields: " + strings.Join(missingFields, ", "))
	}

	return nil
}

func (config *Config) MustValidate() {
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
}

func (config *Config) IsDev() bool {
	return config.Env == constants.ConfigEnvDev
}

func (config *Config) IsStaging() bool {
	return config.Env == constants.ConfigEnvStaging
}

func (config *Config) IsProd() bool {
	return config.Env == constants.ConfigEnvProd
}

func (config *Config) GetDSN() string {
	return config.DSN
}

func (config *Config) GetSecureCookieKey() string {
	return config.SecureCookieKey
}

func (config *Config) GetRedisAddr() string {
	return config.RedisAddr
}

func (config *Config) GetRedisPassword() string {
	return config.RedisPassword
}

func (config *Config) GetRedisDB() int {
	return config.RedisDB
}

// GetCryptBackend returns the backend used to encrypt secrets at rest.
// "local" uses CryptLocalKey, "kms" uses AWS KMS.
func (config *Config) GetCryptBackend() string {
	return config.CryptBackend
}

// GetCryptLocalKey returns the 32-byte AES-256 key used by the local
// crypt backend. Empty when the backend is "kms".
func (config *Config) GetCryptLocalKey() string {
	return config.CryptLocalKey
}

// GetAwsKmsKeyId returns the AWS KMS Key ID used by the kms crypt
// backend. Empty when the backend is "local".
func (config *Config) GetAwsKmsKeyId() string {
	return config.AwsKmsKeyId
}

// GetMailServiceAddr returns the mail service address
func (config *Config) GetMailServiceAddr() string {
	return config.MailServiceAddr
}

// GetAmqpAddr returns the AMQP address
func (config *Config) GetAmqpAddr() string {
	return config.AmqpAddr
}

// GetDomainName returns the domain name
func (config *Config) GetDomainName() string {
	return config.DomainName
}

// GetTotpIssuer returns the label authenticator apps show for this
// deployment's 2FA entries.
//
// TOTP_ISSUER when set, verbatim — including in staging. An operator
// who names the issuer has said what they want it to read, and quietly
// appending "Staging" to it would be us overriding that.
//
// Otherwise derived from DomainName, which Validate already requires,
// so the fallback is never empty in a deployment that starts at all.
// The staging suffix exists on the derived path because both
// environments share DOMAIN_NAME: without it, someone with 2FA on both
// gets two identically-labelled entries in their authenticator and no
// way to tell which code goes where.
//
// Deliberately not run through Validate. An issuer is a display label;
// refusing to boot over one would take out sign-in for everybody to fix
// a cosmetic problem for the people enrolling today.
// GetSiteName returns what this installation calls itself.
//
// Falls back to DomainName, which Validate already requires, so this is
// never empty in a deployment that starts at all. Same shape as
// GetTotpIssuer below, and deliberately not run through Validate for
// the same reason: a display name is not worth refusing to boot over.
//
// Unlike GetTotpIssuer there is no staging suffix. The issuer needs one
// because a person with 2FA on both environments otherwise gets two
// identically-labelled entries in their authenticator; an email already
// says which environment it came from in its links, and "Banu Staging
// password changed" reads like the product is called that.
func (config *Config) GetSiteName() string {
	if config.SiteName != "" {
		return config.SiteName
	}

	return config.DomainName
}

// GetMailFrom returns the From header for outbound notifications, with
// the site name as the display part: `Banu <system@banu.io>`.
//
// mail.Address.String() does the whole job: it RFC 2047-encodes a name
// containing non-ASCII, so a site called "Bänu" arrives readable rather
// than as mojibake, and it applies the quoting rules for a name
// containing a comma or a quote — a comma is the address separator, so
// an unquoted one would split the header into two addresses.
//
// Deliberately NOT pre-encoded with mime.QEncoding. Doing both produces
// an already-ASCII encoded-word that String() then treats as a literal
// name and quotes, so the recipient reads "=?utf-8?q?B=C3=A4nu?=".
//
// NOTE for whoever touches the mail path: this is a HEADER value and is
// not a valid SMTP envelope sender. providers/smtp.go parses the
// address back out for MAIL FROM; see the comment there.
func (config *Config) GetMailFrom() string {
	name := config.GetSiteName()
	if name == "" {
		return config.SystemEmail
	}

	addr := mail.Address{
		Name:    name,
		Address: config.SystemEmail,
	}

	return addr.String()
}

func (config *Config) GetTotpIssuer() string {
	if config.TotpIssuer != "" {
		return config.TotpIssuer
	}

	if config.IsStaging() {
		return fmt.Sprintf("%s Staging", config.DomainName)
	}

	return config.DomainName
}

// GetFileServiceAddr returns the file service address
func (config *Config) GetFileServiceAddr() string {
	return config.FileServiceAddr
}

// GetCompleteDomainName returns the URL of the public marketing site.
//
// NOT the app. In production this is the www host, while the SPA is
// served from the apex — the two are different origins and are not
// interchangeable in a link a person is meant to click. Use
// GetFrontendURL for anything that should land someone inside the
// application.
//
// The distinction is invisible in staging, where both resolve to
// staging.{domain}, so a caller that picks the wrong one here will
// pass every staging test and break in production only.
func (config *Config) GetCompleteDomainName() string {
	if config.IsStaging() {
		return fmt.Sprintf("https://staging.%s", config.DomainName)
	}

	return fmt.Sprintf("https://www.%s", config.DomainName)
}

// GetOAuthIssuer returns the canonical issuer URL of auth-service.
// This value is stamped into the "iss" claim of every JWT minted by
// this service and is enforced as the expected issuer by the
// anahita-libs OAuth middleware in every microservice.
//
// Must be identical across all microservices that share an
// auth-service deployment (e.g. "https://auth.anahita.io").
func (config *Config) GetOAuthIssuer() string {
	return config.OAuthIssuer
}

// GetOAuthJWKSURL returns the URL of auth-service's JWKS endpoint.
// The anahita-libs OAuth middleware fetches public keys from here to
// verify JWT signatures locally.
//
// If unset, NewConfig derives this from the issuer URL by appending
// "/.well-known/jwks.json" — auth-service is its own JWKS publisher in
// the standard OIDC layout.
func (config *Config) GetOAuthJWKSURL() string {
	return config.OAuthJWKSURL
}

// GetOAuthSeedClientPath returns the filesystem path to the JSON file
// containing first-party OAuth client registrations to seed at
// startup. Idempotent — clients already registered are left untouched.
func (config *Config) GetOAuthSeedClientPath() string {
	return config.OAuthSeedClientPath
}

// GetOAuthPrivateKeyPath returns the filesystem path to a PEM-encoded
// RSA private key used for JWT signing. Optional — if empty,
// SeedSigningKey generates a fresh keypair on first run. Used in dev
// to keep tokens valid across service restarts; in production with
// KMS, leave unset.
func (config *Config) GetOAuthPrivateKeyPath() string {
	return config.OAuthPrivateKeyPath
}

// GetOAuthKeyLifetime returns the validity window for newly seeded
// signing keys before they're considered for rotation. Old keys remain
// in the JWKS document until natural expiry so outstanding tokens
// continue to verify.
func (config *Config) GetOAuthKeyLifetime() time.Duration {
	return config.OAuthKeyLifetime
}

// GetOAuthSignerBackend returns the signing backend for JWTs. "local"
// uses a locally generated key, "kms" uses AWS KMS.
func (config *Config) GetOAuthSignerBackend() string {
	return config.OAuthSignerBackend
}

// GetOAuthKMSKeyAlias returns the alias of the AWS KMS key used for signing
// when OAuthSignerBackend is "kms". This should be the alias,
// not the full ARN, e.g. "alias/auth-service-signing-key". The KMS key must
// allow the "kms:Sign" action for the IAM role of auth-service.
func (config *Config) GetOAuthKMSKeyAlias() string {
	return config.OAuthKMSKeyAlias
}

// GetAuthURL returns the public base URL of auth-service for the
// current environment. This is the host where /oauth/authorize,
// /oauth/login, and the IdP session cookie live.
//
// Used to build the second-leg redirect of the cross-domain logout
// flow — the BFF-host /oauth/logout endpoint redirects here so the
// browser delivers a Set-Cookie response targeting the auth host,
// which is the only host that can delete the IdP session cookie.
func (config *Config) GetAuthURL() string {
	switch {
	case config.IsProd():
		return fmt.Sprintf("https://auth.%s", config.DomainName)
	case config.IsStaging():
		return fmt.Sprintf("https://auth.staging.%s", config.DomainName)
	default:
		return "http://localhost"
	}
}

// GetFrontendURL returns the public base URL of the Anahita web app
// for the current environment — the app itself, where the support
// page, pricing, and legal pages live.
//
// Use this, not GetCompleteDomainName, for any link meant to land
// someone inside the application. See the note on that method.
func (config *Config) GetFrontendURL() string {
	switch {
	case config.IsProd():
		return fmt.Sprintf("https://%s", config.DomainName)
	case config.IsStaging():
		return fmt.Sprintf("https://staging.%s", config.DomainName)
	default:
		return "http://localhost:3000"
	}
}

// IsAuthHost reports whether the given request host (e.g. from
// echo.Context.Request().Host) is the auth subdomain for the current
// environment. Strips any port suffix before comparing.
//
// Used by ReadLogout to dispatch to the correct leg of the logout
// redirect chain depending on which host the request landed on.
func (config *Config) IsAuthHost(host string) bool {
	if idx := strings.IndexByte(host, ':'); idx >= 0 {
		host = host[:idx]
	}
	switch {
	case config.IsProd():
		return host == fmt.Sprintf("auth.%s", config.DomainName)
	case config.IsStaging():
		return host == fmt.Sprintf("auth.staging.%s", config.DomainName)
	default:
		return host == "localhost"
	}
}

// GetAllowedAppOrigins returns the list of first-party app origins
// permitted as logout return targets in the current environment.
//
// This is the open-redirect allowlist for /oauth/logout?return_to=…
// — without it, an attacker could craft a logout link that lands the
// user on a phishing page after sign-out. It also feeds the WebAuthn
// RP origin allowlist, which is compared literally rather than by
// suffix, so every host that can run a ceremony has to appear here.
//
// Derived from GetFrontendURL rather than repeating the host string:
// two lists that must agree but are maintained separately will
// eventually disagree, and the failure here is a security boundary
// quietly drifting out of step with the app.
//
// When new apps come online, add their origins here. Keeping this
// in code (rather than runtime config) is deliberate: a typo or
// accidentally permissive env var value would silently weaken the
// security boundary.
func (config *Config) GetAllowedAppOrigins() []string {
	return []string{
		config.GetFrontendURL(),
	}
}

// GetWebAuthnRPID returns the WebAuthn relying-party ID.
//
// EFFECTIVELY PERMANENT ONCE ANYONE ENROLS. Every passkey is
// cryptographically scoped to the RP ID in force when it was created;
// change this and every existing passkey stops being offered, with no
// migration path and no error the person can act on.
//
// Must be a registrable suffix of every origin that runs a ceremony —
// registration happens in the SPA, login on the auth host — which is
// why it is the shared parent domain rather than the auth host itself.
func (config *Config) GetWebAuthnRPID() string {
	switch {
	case config.IsProd():
		// Covers {domain} and any single label beneath it, so both the
		// web app and auth.{domain} can run ceremonies.
		//
		// Note the asymmetry with staging below: a host that is a
		// single label under the apex works here but has no staging
		// equivalent. Anything relying on that would ship untested, so
		// treat it as a constraint rather than an opportunity.
		return config.DomainName
	case config.IsStaging():
		// Distinct from prod on purpose. Both environments share
		// DOMAIN_NAME, so returning it here would give staging and
		// production the same relying party — the browser would offer
		// staging passkeys on the production login page, where they
		// resolve to nothing and fail with no explanation the person
		// can act on.
		//
		// Covers staging.{domain} and auth.staging.{domain}. It does
		// NOT cover a sibling label such as foo-staging.{domain},
		// which is not a subdomain of staging.{domain}.
		//
		// That constraint is worth knowing before adding a staging
		// host: Amplify's managed certificates cover the apex plus a
		// single wildcard level, so foo.staging.{domain} cannot be
		// issued a certificate, and a sibling foo-staging.{domain}
		// falls outside this relying party. Either way, a new app
		// cannot run a WebAuthn ceremony on staging without its own
		// relying party.
		//
		// If that is ever needed, give the app its own relying party
		// scoped to its host — do not change this value. Widening this
		// to {domain} would merge staging and production into one
		// relying party, which is the thing this branch exists to
		// prevent.
		return fmt.Sprintf("staging.%s", config.DomainName)
	default:
		return "localhost"
	}
}
