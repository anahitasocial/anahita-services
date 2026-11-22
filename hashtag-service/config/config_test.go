package config

import (
	"anahita-libs/settings"
	"os"
	"strings"
	"testing"
)

func setEnv(key, value string) func() {
	orig := os.Getenv(key)
	os.Setenv(key, value)
	return func() { os.Setenv(key, orig) }
}

func TestNewConfig_Defaults(t *testing.T) {
	defer setEnv("ENV", "")()
	defer setEnv("SECURE_COOKIE_KEY", "")()
	defer setEnv("VALKEY_ADDR", "")()
	defer setEnv("VALKEY_PASSWORD", "")()
	defer setEnv("VALKEY_DB", "")()
	defer setEnv("FILE_SERVICE_ADDR", "")()
	defer setEnv("POLYMORPHIC_SERVICE_ADDR", "")()
	defer setEnv("AMQP_ADDR", "")()

	cfg := NewConfig()
	if cfg.Env != "dev" {
		t.Errorf("expected default Env to be 'dev', got %s", cfg.Env)
	}
	if cfg.ValkeyAddr != "localhost:6379" {
		t.Errorf("expected default ValkeyAddr to be 'localhost:6379', got %s", cfg.ValkeyAddr)
	}
	if cfg.ValkeyDB != 0 {
		t.Errorf("expected default ValkeyDB to be 0, got %d", cfg.ValkeyDB)
	}
}

func TestConfig_Validate_AllMissing(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing fields, got nil")
	}
	msg := err.Error()
	if !contains(msg, settings.ErrMissingSecureCookieKey) ||
		!contains(msg, settings.ErrMissingFileAddr) ||
		!contains(msg, settings.ErrMissingGraphAddr) ||
		!contains(msg, settings.ErrMissingAmqpAddr) {
		t.Errorf("unexpected error message: %s", msg)
	}
	// Empty is missing, said once, not also the wrong size (roadmap row 170).
	if contains(msg, settings.ErrWrongSizeSecureCookieKey) {
		t.Errorf("an empty key was also reported as the wrong size: %q", msg)
	}
}

func TestConfig_Validate_WrongSecureCookieKeySize(t *testing.T) {
	cfg := &Config{
		Common: settings.Common{
			SecureCookieKey:  "short-key",
			FileServiceAddr:  "file",
			GraphServiceAddr: "graph",
			AmqpAddr:         "amqp",
		},
	}
	err := cfg.Validate()
	if err == nil || !contains(err.Error(), settings.ErrWrongSizeSecureCookieKey) {
		t.Errorf("expected error for wrong secure cookie key size, got %v", err)
	}
}

func TestConfig_Validate_Success(t *testing.T) {
	cfg := &Config{
		Common: settings.Common{
			SecureCookieKey:  "12345678901234567890123456789012",
			FileServiceAddr:  "file",
			GraphServiceAddr: "graph",
			AmqpAddr:         "amqp",
			OAuthIssuer:      "https://auth.example.com",
			OAuthJWKSURL:     "https://auth.example.com/.well-known/jwks.json",
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestConfig_Validate_MissingOAuth(t *testing.T) {
	cfg := &Config{
		Common: settings.Common{
			SecureCookieKey:  "12345678901234567890123456789012",
			FileServiceAddr:  "file",
			GraphServiceAddr: "graph",
			AmqpAddr:         "amqp",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing OAuth fields, got nil")
	}
	if !contains(err.Error(), settings.ErrMissingOAuthIssuer) ||
		!contains(err.Error(), settings.ErrMissingOAuthJWKSURL) {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestNewConfig_DerivesJWKSURLFromIssuer(t *testing.T) {
	defer setEnv("OAUTH_ISSUER", "https://auth.example.com/")()
	defer setEnv("OAUTH_JWKS_URL", "")()

	cfg := NewConfig()
	want := "https://auth.example.com/.well-known/jwks.json"
	if cfg.GetOAuthJWKSURL() != want {
		t.Errorf("expected derived JWKS URL %q, got %q", want, cfg.GetOAuthJWKSURL())
	}
}

func TestConfig_EnvHelpers(t *testing.T) {
	cfg := &Config{Common: settings.Common{Env: "dev"}}
	if !cfg.IsDev() {
		t.Error("IsDev should be true for 'dev'")
	}
	cfg.Env = "staging"
	if !cfg.IsStaging() {
		t.Error("IsStaging should be true for 'staging'")
	}
	cfg.Env = "prod"
	if !cfg.IsProd() {
		t.Error("IsProd should be true for 'prod'")
	}
}

func TestConfig_Getters(t *testing.T) {
	cfg := &Config{
		Common: settings.Common{
			SecureCookieKey:  "key",
			ValkeyAddr:       "addr",
			ValkeyPassword:   "pass",
			ValkeyDB:         2,
			FileServiceAddr:  "file",
			GraphServiceAddr: "graph",
			AmqpAddr:         "amqp",
		},
	}
	if cfg.GetSecureCookieKey() != "key" ||
		cfg.GetValkeyAddr() != "addr" ||
		cfg.GetValkeyPassword() != "pass" ||
		cfg.GetValkeyDB() != 2 ||
		cfg.GetFileServiceAddr() != "file" ||
		cfg.GetGraphServiceAddr() != "graph" ||
		cfg.GetAmqpAddr() != "amqp" {
		t.Error("getter methods returned unexpected values")
	}
}
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
