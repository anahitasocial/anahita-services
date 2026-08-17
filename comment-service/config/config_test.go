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
	defer setEnv("FILE_SERVICE_ADDR", "")()
	defer setEnv("GRAPH_SERVICE_ADDR", "")()
	defer setEnv("AMQP_ADDR", "")()

	cfg := NewConfig()
	if cfg.Env != "dev" {
		t.Errorf("expected default Env to be 'dev', got %s", cfg.Env)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("expected default RedisAddr to be 'localhost:6379', got %s", cfg.RedisAddr)
	}
	if cfg.RedisDB != 0 {
		t.Errorf("expected default RedisDB to be 0, got %d", cfg.RedisDB)
	}
}

func TestConfig_Validate_AllMissing(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing fields, got nil")
	}
	msg := err.Error()
	if !contains(msg, ErrMissingDSN) ||
		!contains(msg, ErrMissingSecureCookieKey) ||
		!contains(msg, ErrWrongSizeSecureCookieKey) ||
		!contains(msg, ErrMissingFileAddr) ||
		!contains(msg, ErrMissingGraphAddr) ||
		!contains(msg, ErrMissingAmqpAddr) {
		t.Errorf("unexpected error message: %s", msg)
	}
}

func TestConfig_Validate_SecureCookieKeyWrongSize(t *testing.T) {
	cfg := &Config{
		DSN:              "dsn",
		SecureCookieKey:  "short",
		FileServiceAddr:  "file",
		GraphServiceAddr: "graph",
		AmqpAddr:         "amqp",
	}
	err := cfg.Validate()
	if err == nil || !contains(err.Error(), ErrWrongSizeSecureCookieKey) {
		t.Errorf("expected error for wrong size secure cookie key, got %v", err)
	}
}

func TestConfig_Validate_Success(t *testing.T) {
	cfg := &Config{
		DSN:              "dsn",
		SecureCookieKey:  "12345678901234567890123456789012",
		FileServiceAddr:  "file",
		GraphServiceAddr: "graph",
		AmqpAddr:         "amqp",
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
		DSN:              "dsn",
		SecureCookieKey:  "12345678901234567890123456789012",
		FileServiceAddr:  "file",
		GraphServiceAddr: "graph",
		AmqpAddr:         "amqp",
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
		FileServiceAddr:  "file",
		GraphServiceAddr: "graph",
		AmqpAddr:         "amqp",
	}
	if cfg.GetDSN() != "dsn" ||
		cfg.GetSecureCookieKey() != "key" ||
		cfg.GetRedisAddr() != "addr" ||
		cfg.GetRedisPassword() != "pass" ||
		cfg.GetRedisDB() != 2 ||
		cfg.GetFileServiceAddr() != "file" ||
		cfg.GetGraphServiceAddr() != "graph" ||
		cfg.GetAmqpAddr() != "amqp" {
		t.Error("getter methods returned unexpected values")
	}
}
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
