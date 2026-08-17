package config

import (
	"os"
	"strings"
	"testing"
)

func setEnvVars(vars map[string]string) func() {
	originals := make(map[string]string)
	for k, v := range vars {
		originals[k] = os.Getenv(k)
		os.Setenv(k, v)
	}
	return func() {
		for k, v := range originals {
			os.Setenv(k, v)
		}
	}
}

func TestNewConfig_Defaults(t *testing.T) {
	cleanup := setEnvVars(map[string]string{
		"ENV":                "",
		"DSN":                "",
		"SECURE_COOKIE_KEY":  "",
		"REDIS_ADDR":         "",
		"REDIS_PASSWORD":     "",
		"REDIS_DB":           "",
		"FILE_SERVICE_ADDR":  "",
		"GRAPH_SERVICE_ADDR": "",
		"AMQP_ADDR":          "",
	})
	defer cleanup()

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
	if !contains(msg, ErrMissingSecureCookieKey) ||
		!contains(msg, ErrWrongSizeSecureCookieKey) ||
		!contains(msg, ErrMissingDSN) ||
		!contains(msg, ErrMissingFileAddr) ||
		!contains(msg, ErrMissingGraphAddr) ||
		!contains(msg, ErrMissingAmqpAddr) {
		t.Errorf("missing expected error messages, got: %s", msg)
	}
}

func TestConfig_Validate_SecureCookieKeyWrongSize(t *testing.T) {
	cfg := &Config{
		SecureCookieKey:  "shortkey",
		DSN:              "dsn",
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
		SecureCookieKey:  "12345678901234567890123456789012",
		DSN:              "dsn",
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
		SecureCookieKey:  "12345678901234567890123456789012",
		DSN:              "dsn",
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
	cleanup := setEnvVars(map[string]string{
		"OAUTH_ISSUER":   "https://auth.example.com/",
		"OAUTH_JWKS_URL": "",
	})
	defer cleanup()

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
