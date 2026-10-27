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

func unsetEnv(key string) func() {
	orig := os.Getenv(key)
	os.Unsetenv(key)
	return func() { os.Setenv(key, orig) }
}

func TestNewConfig_Defaults(t *testing.T) {
	defer setEnv("ENV", "")()
	defer setEnv("SMTP_USER", "user")()
	defer setEnv("SMTP_PASSWORD", "pass")()
	defer setEnv("SMTP_HOST", "smtp.example.com")()
	defer setEnv("SMTP_PORT", "2525")()
	defer setEnv("SMTP_SECURE", "true")()

	cfg := NewConfig()

	if cfg.Env == "" {
		t.Error("expected Env to be set")
	}
	if cfg.Smtp.SmtpUser != "user" {
		t.Errorf("expected SmtpUser=user, got %s", cfg.Smtp.SmtpUser)
	}
	if cfg.Smtp.SmtpPassword != "pass" {
		t.Errorf("expected SmtpPassword=pass, got %s", cfg.Smtp.SmtpPassword)
	}
	if cfg.Smtp.SmtpHost != "smtp.example.com" {
		t.Errorf("expected SmtpHost=smtp.example.com, got %s", cfg.Smtp.SmtpHost)
	}
	if cfg.Smtp.SmtpPort != 2525 {
		t.Errorf("expected SmtpPort=2525, got %d", cfg.Smtp.SmtpPort)
	}
	if !cfg.Smtp.SmtpSecure {
		t.Errorf("expected SmtpSecure=true, got false")
	}
}

func TestConfig_Validate_AllFieldsPresent(t *testing.T) {
	defer setEnv("ENV", "prod")()
	defer setEnv("SMTP_USER", "user")()
	defer setEnv("SMTP_PASSWORD", "pass")()
	defer setEnv("SMTP_HOST", "host")()
	defer setEnv("SMTP_PORT", "25")()

	cfg := NewConfig()
	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// SMTP is the only transport, so validation no longer switches on a provider.
// Host and port are required; credentials are not — an internal relay or a
// local mail sink that accepts unauthenticated mail is an ordinary setup, and
// demanding a username would make it unusable.
func TestConfig_Validate_SmtpRules(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "host and port are enough",
			cfg:     Config{Env: "prod", Smtp: Smtp{SmtpHost: "smtp.example.com", SmtpPort: 587}},
			wantErr: false,
		},
		{
			name:    "credentials are optional",
			cfg:     Config{Env: "prod", Smtp: Smtp{SmtpHost: "mailpit", SmtpPort: 1025}},
			wantErr: false,
		},
		{
			name:    "no host",
			cfg:     Config{Env: "prod", Smtp: Smtp{SmtpPort: 587}},
			wantErr: true,
		},
		{
			name:    "no port",
			cfg:     Config{Env: "prod", Smtp: Smtp{SmtpHost: "smtp.example.com"}},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

// The redirect is what stops a development machine mailing real people, so
// booting without a target in development is refused — an empty override would
// blank the recipient list instead. Production needs no such value.
func TestConfig_Validate_DeveloperEmailRequiredInDevOnly(t *testing.T) {
	base := Config{
		Smtp: Smtp{SmtpHost: "smtp.example.com", SmtpPort: 587},
	}

	dev := base
	dev.Env = "dev"
	if err := dev.Validate(); err == nil {
		t.Error("expected dev without DEVELOPER_EMAIL to fail, got nil")
	}

	dev.DeveloperEmail = "developer@example.com"
	if err := dev.Validate(); err != nil {
		t.Errorf("expected dev with DEVELOPER_EMAIL to pass, got %v", err)
	}

	for _, env := range []string{"staging", "prod"} {
		other := base
		other.Env = env
		if err := other.Validate(); err != nil {
			t.Errorf("expected %s without DEVELOPER_EMAIL to pass, got %v", env, err)
		}
	}
}

func TestConfig_Validate_MissingFields(t *testing.T) {
	unset := []func(){}
	unset = append(unset, unsetEnv("SMTP_USER"))
	unset = append(unset, unsetEnv("SMTP_PASSWORD"))
	unset = append(unset, unsetEnv("SMTP_HOST"))
	unset = append(unset, unsetEnv("SMTP_PORT"))
	defer func() {
		for _, f := range unset {
			f()
		}
	}()

	cfg := NewConfig()
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing fields, got nil")
	}
}

func TestConfig_IsDev_IsStaging_IsProd(t *testing.T) {
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
		Smtp: Smtp{
			SmtpUser:     "user",
			SmtpPassword: "pass",
			SmtpHost:     "host",
			SmtpPort:     123,
			SmtpSecure:   true,
		},
		DeveloperEmail: "developer@example.com",
	}
	if cfg.GetDeveloperEmail() != "developer@example.com" {
		t.Error("GetDeveloperEmail failed")
	}
	if cfg.GetSmtpUser() != "user" {
		t.Error("GetSmtpUser failed")
	}
	if cfg.GetSmtpPassword() != "pass" {
		t.Error("GetSmtpPassword failed")
	}
	if cfg.GetSmtpHost() != "host" {
		t.Error("GetSmtpHost failed")
	}
	if cfg.GetSmtpPort() != 123 {
		t.Error("GetSmtpPort failed")
	}
	if !cfg.GetSmtpSecure() {
		t.Error("GetSmtpSecure failed")
	}
}

// SMTP_SECURE selects whether the session is encrypted from the first byte or
// upgraded in place. An unparseable value used to fall back to false silently,
// so SMTP_SECURE="ssl" — a completely reasonable thing to write, and the
// opposite of what it did — ran with STARTTLS and said nothing.
func TestConfig_Validate_RejectsUnparseableSmtpSecure(t *testing.T) {
	defer setEnv("ENV", "prod")()
	defer setEnv("SMTP_HOST", "smtp.example.com")()
	defer setEnv("SMTP_PORT", "587")()

	for _, value := range []string{"ssl", "tls", "yes", "starttls"} {
		t.Run(value, func(t *testing.T) {
			defer setEnv("SMTP_SECURE", value)()

			cfg := NewConfig()
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("SMTP_SECURE=%q was accepted", value)
			}
			if !strings.Contains(err.Error(), "SMTP_SECURE") {
				t.Errorf("error does not name the setting: %v", err)
			}
		})
	}
}

func TestConfig_Validate_AcceptsBooleanSmtpSecure(t *testing.T) {
	defer setEnv("ENV", "prod")()
	defer setEnv("SMTP_HOST", "smtp.example.com")()
	defer setEnv("SMTP_PORT", "587")()

	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			defer setEnv("SMTP_SECURE", tc.value)()

			cfg := NewConfig()
			if err := cfg.Validate(); err != nil {
				t.Fatalf("SMTP_SECURE=%q rejected: %v", tc.value, err)
			}
			if cfg.Smtp.SmtpSecure != tc.want {
				t.Errorf("SmtpSecure = %v, want %v", cfg.Smtp.SmtpSecure, tc.want)
			}
		})
	}
}

// Unset is fine and means STARTTLS, which is what most providers expect.
func TestConfig_Validate_SmtpSecureMayBeUnset(t *testing.T) {
	defer setEnv("ENV", "prod")()
	defer setEnv("SMTP_HOST", "smtp.example.com")()
	defer setEnv("SMTP_PORT", "587")()
	defer unsetEnv("SMTP_SECURE")()

	cfg := NewConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unset SMTP_SECURE rejected: %v", err)
	}
	if cfg.Smtp.SmtpSecure {
		t.Error("unset SMTP_SECURE should mean STARTTLS")
	}
}

// An email logo must be an absolute URL: a relative path resolves against
// nothing in a mail client and renders as a broken image everywhere.
func TestGetSiteLogoURLRejectsAnythingRelative(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"https://auth.example.com/static/images/logo.png", "https://auth.example.com/static/images/logo.png"},
		{"http://localhost/static/images/logo.png", "http://localhost/static/images/logo.png"},
		// Rejected: each of these renders as a broken image in a mail client.
		{"/static/images/logo.png", ""},
		{"static/images/logo.png", ""},
		{"//auth.example.com/logo.png", ""},
		// Empty is a supported state — the template renders the site name.
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			cfg := &Config{SiteLogoURL: tc.value}
			if got := cfg.GetSiteLogoURL(); got != tc.want {
				t.Errorf("GetSiteLogoURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A blank header looks broken rather than unbranded, so an unset site name
// falls back to something neutral.
func TestGetSiteNameNeverReturnsEmpty(t *testing.T) {
	cfg := &Config{}
	if cfg.GetSiteName() == "" {
		t.Error("GetSiteName() returned empty; the message header would be blank")
	}

	cfg.SiteName = "Example"
	if got := cfg.GetSiteName(); got != "Example" {
		t.Errorf("GetSiteName() = %q, want Example", got)
	}
}
