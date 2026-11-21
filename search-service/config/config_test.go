package config

import (
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
	defer setEnv("DSN", "")()
	defer setEnv("SECURE_COOKIE_KEY", "")()
	defer setEnv("VALKEY_ADDR", "")()
	defer setEnv("VALKEY_PASSWORD", "")()
	defer setEnv("VALKEY_DB", "")()
	defer setEnv("FILE_SERVICE_ADDR", "")()
	defer setEnv("GRAPH_SERVICE_ADDR", "")()

	cfg := NewConfig()
	if cfg.Env != "dev" {
		t.Errorf("expected Env to be 'dev', got %q", cfg.Env)
	}
	if cfg.ValkeyAddr != "localhost:6379" {
		t.Errorf("expected ValkeyAddr to be 'localhost:6379', got %q", cfg.ValkeyAddr)
	}
	if cfg.ValkeyDB != 0 {
		t.Errorf("expected ValkeyDB to be 0, got %d", cfg.ValkeyDB)
	}
}

func TestConfig_Validate_AllMissing(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing fields, got nil")
	}
	msg := err.Error()
	if !contains(msg, ErrMissingSecureCookieKey) ||
		!contains(msg, ErrWrongSizeSecureCookieKey) ||
		!contains(msg, ErrMissingDSN) ||
		!contains(msg, ErrMissingFileAddr) ||
		!contains(msg, ErrMissingGraphAddr) {
		t.Errorf("unexpected error message: %v", msg)
	}
}

func TestConfig_Validate_SecureCookieKeyWrongSize(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "short",
		DSN:              "dsn",
		FileServiceAddr:  "file",
		GraphServiceAddr: "graph",
	}
	err := cfg.Validate()
	if err == nil || !contains(err.Error(), ErrWrongSizeSecureCookieKey) {
		t.Errorf("expected error for wrong size secure cookie key, got %v", err)
	}
}

func TestConfig_Validate_Success(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "12345678901234567890123456789012",
		DSN:              "dsn",
		FileServiceAddr:  "file",
		GraphServiceAddr: "graph",
		OAuthIssuer:      "https://auth.example.com",
		OAuthJWKSURL:     "https://auth.example.com/.well-known/jwks.json",
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestConfig_Validate_MissingOAuth(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "12345678901234567890123456789012",
		DSN:              "dsn",
		FileServiceAddr:  "file",
		GraphServiceAddr: "graph",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing OAuth fields, got nil")
	}
	if !contains(err.Error(), ErrMissingOAuthIssuer) ||
		!contains(err.Error(), ErrMissingOAuthJWKSURL) {
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

func TestConfig_IsEnv(t *testing.T) {
	cfg := &Config{Env: "dev"}
	if !cfg.IsDev() {
		t.Error("expected IsDev to be true")
	}
	cfg.Env = "staging"
	if !cfg.IsStaging() {
		t.Error("expected IsStaging to be true")
	}
	cfg.Env = "prod"
	if !cfg.IsProd() {
		t.Error("expected IsProd to be true")
	}
}

func TestConfig_Getters(t *testing.T) {
	cfg := &Config{
		DSN:              "dsn",
		SecureCookieKey:  "key",
		ValkeyAddr:       "addr",
		ValkeyPassword:   "pass",
		ValkeyDB:         42,
		FileServiceAddr:  "file",
		GraphServiceAddr: "graph",
	}
	if cfg.GetDSN() != "dsn" ||
		cfg.GetSecureCookieKey() != "key" ||
		cfg.GetValkeyAddr() != "addr" ||
		cfg.GetValkeyPassword() != "pass" ||
		cfg.GetValkeyDB() != 42 ||
		cfg.GetFileServiceAddr() != "file" ||
		cfg.GetGraphServiceAddr() != "graph" {
		t.Error("getter methods returned unexpected values")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
