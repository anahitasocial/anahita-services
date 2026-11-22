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
	defer setEnv("SECURE_COOKIE_KEY", "")()
	defer setEnv("VALKEY_ADDR", "")()
	defer setEnv("VALKEY_PASSWORD", "")()
	defer setEnv("VALKEY_DB", "")()
	defer setEnv("GRAPH_SERVICE_ADDR", "")()
	defer setEnv("FILE_SERVICE_ADDR", "")()

	cfg := NewConfig()
	if cfg.Env != "dev" {
		t.Errorf("expected Env=dev, got %s", cfg.Env)
	}
	if cfg.ValkeyAddr != "localhost:6379" {
		t.Errorf("expected ValkeyAddr=localhost:6379, got %s", cfg.ValkeyAddr)
	}
	if cfg.ValkeyDB != 0 {
		t.Errorf("expected ValkeyDB=0, got %d", cfg.ValkeyDB)
	}
}

func TestConfig_Validate_AllMissing(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !contains(msg, ErrMissingSecureCookieKey) {
		t.Errorf("expected error to contain %q, got %q", ErrMissingSecureCookieKey, msg)
	}
	// Empty is missing, said once, not also the wrong size (roadmap row 170).
	if contains(msg, ErrWrongSizeSecureCookieKey) {
		t.Errorf("an empty key was also reported as the wrong size: %q", msg)
	}
	if !contains(msg, ErrMissingGraphAddr) {
		t.Errorf("expected error to contain %q, got %q", ErrMissingGraphAddr, msg)
	}
	if !contains(msg, ErrMissingFileAddr) {
		t.Errorf("expected error to contain %q, got %q", ErrMissingFileAddr, msg)
	}
}

func TestConfig_Validate_WrongCookieKeySize(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "short-key",
		GraphServiceAddr: "graph",
		FileServiceAddr:  "file",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), ErrWrongSizeSecureCookieKey) {
		t.Errorf("expected error to contain %q, got %q", ErrWrongSizeSecureCookieKey, err.Error())
	}
}

func TestConfig_Validate_Success(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "12345678901234567890123456789012",
		DSN:              "user@tcp(db)/anahita",
		GraphServiceAddr: "graph",
		FileServiceAddr:  "file",
		OAuthIssuer:      "https://auth.example.com",
		OAuthJWKSURL:     "https://auth.example.com/.well-known/jwks.json",
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestConfig_Validate_MissingOAuth(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "12345678901234567890123456789012",
		GraphServiceAddr: "graph",
		FileServiceAddr:  "file",
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

func TestConfig_EnvChecks(t *testing.T) {
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

func TestConfig_Getters(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "key",
		ValkeyAddr:       "addr",
		ValkeyPassword:   "pass",
		ValkeyDB:         42,
		GraphServiceAddr: "graph",
		FileServiceAddr:  "file",
	}
	if cfg.GetSecureCookieKey() != "key" {
		t.Error("GetSecureCookieKey failed")
	}
	if cfg.GetValkeyAddr() != "addr" {
		t.Error("GetValkeyAddr failed")
	}
	if cfg.GetValkeyPassword() != "pass" {
		t.Error("GetValkeyPassword failed")
	}
	if cfg.GetValkeyDB() != 42 {
		t.Error("GetValkeyDB failed")
	}
	if cfg.GetGraphServiceAddr() != "graph" {
		t.Error("GetGraphServiceAddr failed")
	}
	if cfg.GetFileServiceAddr() != "file" {
		t.Error("GetFileServiceAddr failed")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
