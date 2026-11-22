package config

import (
	"anahita-libs/settings"
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
		"VALKEY_ADDR":        "",
		"VALKEY_PASSWORD":    "",
		"VALKEY_DB":          "",
		"ACTOR_SERVICE_ADDR": "",
		"FILE_SERVICE_ADDR":  "",
		"GRAPH_SERVICE_ADDR": "",
		"AMQP_ADDR":          "",
	})
	defer cleanup()

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

func TestConfig_Validate_MissingFields(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing fields, got nil")
	}
	msg := err.Error()
	if !contains(msg, settings.ErrMissingSecureCookieKey) {
		t.Errorf("expected error to contain %q, got %q", settings.ErrMissingSecureCookieKey, msg)
	}
	// Empty is missing, said once, not also the wrong size (roadmap row 170).
	if contains(msg, settings.ErrWrongSizeSecureCookieKey) {
		t.Errorf("an empty key was also reported as the wrong size: %q", msg)
	}
	if !contains(msg, settings.ErrMissingDSN) {
		t.Errorf("expected error to contain %q, got %q", settings.ErrMissingDSN, msg)
	}
	if !contains(msg, settings.ErrMissingFileAddr) {
		t.Errorf("expected error to contain %q, got %q", settings.ErrMissingFileAddr, msg)
	}
	if !contains(msg, settings.ErrMissingGraphAddr) {
		t.Errorf("expected error to contain %q, got %q", settings.ErrMissingGraphAddr, msg)
	}
	if !contains(msg, settings.ErrMissingAmqpAddr) {
		t.Errorf("expected error to contain %q, got %q", settings.ErrMissingAmqpAddr, msg)
	}
}

func TestConfig_Validate_WrongSecureCookieKeyLength(t *testing.T) {
	cfg := &Config{
		Common: settings.Common{
			SecureCookieKey:  "short",
			DSN:              "dsn",
			FileServiceAddr:  "file",
			GraphServiceAddr: "graph",
			AmqpAddr:         "amqp",
		},
	}
	err := cfg.Validate()
	if err == nil || !contains(err.Error(), settings.ErrWrongSizeSecureCookieKey) {
		t.Errorf("expected error for wrong secure cookie key length, got %v", err)
	}
}

func TestConfig_Validate_Success(t *testing.T) {
	cfg := &Config{
		Common: settings.Common{
			SecureCookieKey:  "12345678901234567890123456789012",
			DSN:              "dsn",
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
			DSN:              "dsn",
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

func TestConfig_EnvHelpers(t *testing.T) {
	cfg := &Config{Common: settings.Common{Env: "dev"}}
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
		Common: settings.Common{
			DSN:              "dsn",
			SecureCookieKey:  "key",
			ValkeyAddr:       "addr",
			ValkeyPassword:   "pass",
			ValkeyDB:         2,
			FileServiceAddr:  "file",
			GraphServiceAddr: "graph",
			AmqpAddr:         "amqp",
		},
	}
	if cfg.GetDSN() != "dsn" {
		t.Error("GetDSN failed")
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
	if cfg.GetValkeyDB() != 2 {
		t.Error("GetValkeyDB failed")
	}
	if cfg.GetFileServiceAddr() != "file" {
		t.Error("GetFileServiceAddr failed")
	}
	if cfg.GetGraphServiceAddr() != "graph" {
		t.Error("GetGraphServiceAddr failed")
	}
	if cfg.GetAmqpAddr() != "amqp" {
		t.Error("GetAmqpAddr failed")
	}
}
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
