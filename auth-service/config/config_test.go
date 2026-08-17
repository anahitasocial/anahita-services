package config

import (
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
		OAuthScopesPath:     defaultOAuthScopesPath,
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
