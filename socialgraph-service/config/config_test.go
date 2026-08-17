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
	defer setEnv("REDIS_ADDR", "")()
	defer setEnv("REDIS_PASSWORD", "")()
	defer setEnv("REDIS_DB", "")()
	defer setEnv("GRAPH_SERVICE_ADDR", "")()
	defer setEnv("FILE_SERVICE_ADDR", "")()

	cfg := NewConfig()
	if cfg.Env != "dev" {
		t.Errorf("expected Env to be 'dev', got %q", cfg.Env)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("expected RedisAddr to be 'localhost:6379', got %q", cfg.RedisAddr)
	}
	if cfg.RedisDB != 0 {
		t.Errorf("expected RedisDB to be 0, got %d", cfg.RedisDB)
	}
}

func TestConfig_Validate_AllMissing(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !contains(msg, ErrMissingSecureCookieKey) ||
		!contains(msg, ErrWrongSizeSecureCookieKey) ||
		!contains(msg, ErrMissingDSN) ||
		!contains(msg, ErrMissingGraphAddr) ||
		!contains(msg, ErrMissingFileAddr) {
		t.Errorf("missing expected error messages, got: %s", msg)
	}
}

func TestConfig_Validate_SecureCookieKeyWrongSize(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "short",
		DSN:              "dsn",
		GraphServiceAddr: "graph",
		FileServiceAddr:  "file",
	}
	err := cfg.Validate()
	if err == nil || !contains(err.Error(), ErrWrongSizeSecureCookieKey) {
		t.Errorf("expected wrong size secure cookie key error, got %v", err)
	}
}

func TestConfig_Validate_Success(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "12345678901234567890123456789012",
		DSN:              "dsn",
		GraphServiceAddr: "graph",
		FileServiceAddr:  "file",
		OAuthIssuer:      "https://auth.example.com",
		OAuthJWKSURL:     "https://auth.example.com/.well-known/jwks.json",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestConfig_Validate_MissingOAuth(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "12345678901234567890123456789012",
		DSN:              "dsn",
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

func TestConfig_EnvHelpers(t *testing.T) {
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
		RedisAddr:        "addr",
		RedisPassword:    "pass",
		RedisDB:          2,
		GraphServiceAddr: "graph",
		FileServiceAddr:  "file",
	}
	if cfg.GetDSN() != "dsn" ||
		cfg.GetSecureCookieKey() != "key" ||
		cfg.GetRedisAddr() != "addr" ||
		cfg.GetRedisPassword() != "pass" ||
		cfg.GetRedisDB() != 2 ||
		cfg.GetGraphServiceAddr() != "graph" ||
		cfg.GetFileServiceAddr() != "file" {
		t.Error("getter methods returned unexpected values")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
