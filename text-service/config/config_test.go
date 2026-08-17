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
	defer setEnv("REDIS_ADDR", "")()
	defer setEnv("REDIS_PASSWORD", "")()
	defer setEnv("REDIS_DB", "")()
	defer setEnv("GRAPH_SERVICE_ADDR", "")()
	defer setEnv("FILE_SERVICE_ADDR", "")()

	cfg := NewConfig()
	if cfg.Env != "dev" {
		t.Errorf("expected Env to be 'dev', got %s", cfg.Env)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("expected RedisAddr to be 'localhost:6379', got %s", cfg.RedisAddr)
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
	if !contains(msg, ErrMissingSecureCookieKey) {
		t.Errorf("expected error to contain %q, got %q", ErrMissingSecureCookieKey, msg)
	}
	if !contains(msg, ErrWrongSizeSecureCookieKey) {
		t.Errorf("expected error to contain %q, got %q", ErrWrongSizeSecureCookieKey, msg)
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
		GraphServiceAddr: "graph",
		FileServiceAddr:  "file",
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
		SecureCookieKey:  "key",
		RedisAddr:        "addr",
		RedisPassword:    "pass",
		RedisDB:          2,
		GraphServiceAddr: "graph",
		FileServiceAddr:  "file",
	}
	if cfg.GetSecureCookieKey() != "key" {
		t.Error("GetSecureCookieKey failed")
	}
	if cfg.GetRedisAddr() != "addr" {
		t.Error("GetRedisAddr failed")
	}
	if cfg.GetRedisPassword() != "pass" {
		t.Error("GetRedisPassword failed")
	}
	if cfg.GetRedisDB() != 2 {
		t.Error("GetRedisDB failed")
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
