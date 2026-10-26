package config

import (
	"net/mail"
	"os"
	"strings"
	"testing"
	"time"

	"anahita-libs/constants"
)

func setEnv(key, value string) func() {
	orig := os.Getenv(key)
	os.Setenv(key, value)
	return func() { os.Setenv(key, orig) }
}

func unsetEnv(key string) func() {
	orig := os.Getenv(key)
	os.Unsetenv(key)
	return func() { os.Setenv(key, orig) }
}

func TestNewConfigDefaults(t *testing.T) {
	defer setEnv("ENV", "")()
	defer unsetEnv("DSN")()
	defer unsetEnv("SECURE_COOKIE_KEY")()
	defer unsetEnv("REDIS_ADDR")()
	defer unsetEnv("REDIS_PASSWORD")()
	defer unsetEnv("REDIS_DB")()
	defer unsetEnv("FILE_SERVICE_ADDR")()

	cfg := NewConfig()
	if cfg.Env == "" {
		t.Error("expected default Env to be set")
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("expected default RedisAddr to be localhost:6379, got %s", cfg.RedisAddr)
	}
	if cfg.RedisDB != 0 {
		t.Errorf("expected default RedisDB to be 0, got %d", cfg.RedisDB)
	}
}

// validConfig returns a Config that passes Validate, for tests that
// want to invalidate exactly one field.
func validConfig() *Config {
	return &Config{
		DSN:                 "dsn",
		SecureCookieKey:     "12345678901234567890123456789012",
		RedisAddr:           "localhost:6379",
		DomainName:          "example.com",
		SystemEmail:         "system@example.com",
		SupportEmail:        "support@example.com",
		MailServiceAddr:     "mail:50051",
		FileServiceAddr:     "file:50051",
		AmqpAddr:            "amqp://localhost:5672",
		CryptBackend:        constants.CryptBackendLocal,
		CryptLocalKey:       "abcdefghijklmnopqrstuvwxyz012345",
		OAuthIssuer:         "https://auth.example.com",
		OAuthJWKSURL:        "https://auth.example.com/.well-known/jwks.json",
		OAuthSeedClientPath: defaultOAuthSeedClientPath,
		OAuthKeyLifetime:    24 * time.Hour,
		OAuthSignerBackend:  constants.OAuthSignerBackendLocal,
	}
}

func TestValidate_AllMissing(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
	msg := err.Error()
	if !contains(msg, ErrMissingDSN) ||
		!contains(msg, ErrMissingSecureCookieKey) ||
		!contains(msg, ErrWrongSizeSecureCookieKey) {
		t.Errorf("unexpected error message: %s", msg)
	}
}

func TestValidate_WrongSecureCookieKeyLength(t *testing.T) {
	cfg := validConfig()
	cfg.SecureCookieKey = "short"

	err := cfg.Validate()
	if err == nil || !contains(err.Error(), ErrWrongSizeSecureCookieKey) {
		t.Errorf("expected error for wrong secure cookie key length, got %v", err)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	err := validConfig().Validate()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// The default crypt backend is local, so a deployment with no AWS at
// all must validate. AWS_KMS_KEY_ID is only required by the kms backend.
func TestNewConfig_CryptBackendDefaultsToLocal(t *testing.T) {
	defer unsetEnv("CRYPT_BACKEND")()

	if got := NewConfig().CryptBackend; got != constants.CryptBackendLocal {
		t.Errorf("expected default crypt backend %q, got %q", constants.CryptBackendLocal, got)
	}
}

func TestNewConfig_CryptBackendNormalizesCase(t *testing.T) {
	defer setEnv("CRYPT_BACKEND", "KMS")()

	if got := NewConfig().CryptBackend; got != constants.CryptBackendKMS {
		t.Errorf("expected %q, got %q", constants.CryptBackendKMS, got)
	}
}

func TestValidate_LocalCryptDoesNotRequireKmsKeyId(t *testing.T) {
	cfg := validConfig()
	cfg.AwsKmsKeyId = ""

	if err := cfg.Validate(); err != nil {
		t.Errorf("local backend should validate without a KMS key id, got %v", err)
	}
}

func TestValidate_LocalCryptRequiresKey(t *testing.T) {
	cfg := validConfig()
	cfg.CryptLocalKey = ""

	err := cfg.Validate()
	if err == nil || !contains(err.Error(), ErrMissingCryptLocalKey) {
		t.Errorf("expected error for missing crypt local key, got %v", err)
	}

	cfg.CryptLocalKey = "short"
	err = cfg.Validate()
	if err == nil || !contains(err.Error(), ErrWrongSizeCryptLocalKey) {
		t.Errorf("expected error for wrong crypt local key length, got %v", err)
	}
}

func TestValidate_KmsCryptRequiresKeyId(t *testing.T) {
	cfg := validConfig()
	cfg.CryptBackend = constants.CryptBackendKMS
	cfg.CryptLocalKey = ""

	err := cfg.Validate()
	if err == nil || !contains(err.Error(), ErrMissingAwsKmsKeyId) {
		t.Errorf("expected error for missing KMS key id, got %v", err)
	}

	cfg.AwsKmsKeyId = "key-id"
	if err := cfg.Validate(); err != nil {
		t.Errorf("kms backend should validate without a local key, got %v", err)
	}
}

func TestValidate_UnknownCryptBackend(t *testing.T) {
	cfg := validConfig()
	cfg.CryptBackend = "vault"

	err := cfg.Validate()
	if err == nil || !contains(err.Error(), "CRYPT_BACKEND") {
		t.Errorf("expected error naming CRYPT_BACKEND, got %v", err)
	}
}

func TestIsDevStagingProd(t *testing.T) {
	cfg := &Config{Env: "dev"}
	if !cfg.IsDev() {
		t.Error("expected IsDev true")
	}
	cfg.Env = "staging"
	if !cfg.IsStaging() {
		t.Error("expected IsStaging true")
	}
	cfg.Env = "prod"
	if !cfg.IsProd() {
		t.Error("expected IsProd true")
	}
}

func TestGetters(t *testing.T) {
	cfg := &Config{
		DSN:             "dsn",
		SecureCookieKey: "key",
		RedisAddr:       "addr",
		RedisPassword:   "pass",
		RedisDB:         2,
		FileServiceAddr: "fileaddr",
	}
	if cfg.GetDSN() != "dsn" ||
		cfg.GetSecureCookieKey() != "key" ||
		cfg.GetRedisAddr() != "addr" ||
		cfg.GetRedisPassword() != "pass" ||
		cfg.GetRedisDB() != 2 ||
		cfg.GetFileServiceAddr() != "fileaddr" {
		t.Error("getter methods returned unexpected values")
	}
}
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// The site name appears in every notification and as the From display name,
// so an installation that never sets SITE_NAME must still read sensibly.
func TestGetSiteNameFallsBackToDomain(t *testing.T) {
	cfg := &Config{DomainName: "banu.io"}
	if got := cfg.GetSiteName(); got != "banu.io" {
		t.Errorf("GetSiteName() = %q, want the domain as a fallback", got)
	}

	cfg.SiteName = "Banu"
	if got := cfg.GetSiteName(); got != "Banu" {
		t.Errorf("GetSiteName() = %q, want Banu", got)
	}
}

func TestGetSiteLogoURLFallsBackToAnahitasMark(t *testing.T) {
	cfg := &Config{}
	if got := cfg.GetSiteLogoURL(); got != "/static/images/logo.svg" {
		t.Errorf("GetSiteLogoURL() = %q, want Anahita's mark as a fallback", got)
	}

	cfg.SiteLogoURL = "https://banu.io/logo.png"
	if got := cfg.GetSiteLogoURL(); got != "https://banu.io/logo.png" {
		t.Errorf("GetSiteLogoURL() = %q, want SITE_LOGO_URL", got)
	}
}

// GetMailFrom output goes into a mail header, so it has to parse as an
// address — and the SMTP provider parses it back out for the envelope, where
// a display name is a syntax error.
func TestGetMailFromIsAParseableAddress(t *testing.T) {
	cases := []struct {
		name     string
		siteName string
		wantName string
	}{
		{"plain", "Banu", "Banu"},
		// Non-ASCII must survive RFC 2047 encoding and decoding.
		{"non-ascii", "Bänu", "Bänu"},
		{"cjk", "バヌ", "バヌ"},
		// A comma is the address separator: unquoted, this would parse as two
		// addresses and the header would be malformed.
		{"comma", "Banu, the site", "Banu, the site"},
		{"quote", `Banu "the site"`, `Banu "the site"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{DomainName: "banu.io", SiteName: tc.siteName, SystemEmail: "system@banu.io"}

			addr, err := mail.ParseAddress(cfg.GetMailFrom())
			if err != nil {
				t.Fatalf("GetMailFrom() = %q, which does not parse: %v", cfg.GetMailFrom(), err)
			}
			if addr.Address != "system@banu.io" {
				t.Errorf("address = %q, want system@banu.io", addr.Address)
			}

			// ParseAddress decodes RFC 2047 for us, so the display name should
			// come back as it went in.
			if addr.Name != tc.wantName {
				t.Errorf("display name = %q, want %q", addr.Name, tc.wantName)
			}
		})
	}
}

// With no site name and no domain there is nothing to display, and the bare
// address is still valid.
func TestGetMailFromWithoutASiteName(t *testing.T) {
	cfg := &Config{SystemEmail: "system@banu.io"}

	if got := cfg.GetMailFrom(); got != "system@banu.io" {
		t.Errorf("GetMailFrom() = %q, want the bare address", got)
	}
}

func TestLocationsFromIsAdministratorsUnlessSet(t *testing.T) {
	if got := (&Config{}).GetLocationsFrom(); got != "administrators" {
		t.Errorf("left out: %q; want administrators, as location-service has it", got)
	}
	if got := (&Config{LocationsFrom: "Registered"}).GetLocationsFrom(); got != "registered" {
		t.Errorf("set: %q; want registered", got)
	}
}
