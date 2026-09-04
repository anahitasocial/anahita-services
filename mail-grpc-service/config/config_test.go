package config

import (
	"os"
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
	defer setEnv("MAILER_PROVIDER", "smtp")()
	defer setEnv("AWS_REGION", "us-east-1")()
	defer setEnv("SMTP_USER", "user")()
	defer setEnv("SMTP_PASSWORD", "pass")()
	defer setEnv("SMTP_HOST", "smtp.example.com")()
	defer setEnv("SMTP_PORT", "2525")()
	defer setEnv("SMTP_SECURE", "true")()

	cfg := NewConfig()

	if cfg.Env == "" {
		t.Error("expected Env to be set")
	}
	if cfg.MailerProvider != "smtp" {
		t.Errorf("expected MailerProvider=smtp, got %s", cfg.MailerProvider)
	}
	if cfg.Aws.Region != "us-east-1" {
		t.Errorf("expected Aws.Region=us-east-1, got %s", cfg.Aws.Region)
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
	defer setEnv("MAILER_PROVIDER", "smtp")()
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

// Validation only asks for what the selected provider uses. Requiring SMTP
// credentials unconditionally meant a Mailgun or SES deployment could not start
// without inventing values it would never read.
func TestConfig_Validate_PerProvider(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "mailgun needs no smtp",
			cfg:     Config{Env: "prod", MailerProvider: "mailgun", Mailgun: Mailgun{ApiKey: "k", Domain: "d"}},
			wantErr: false,
		},
		{
			name:    "mailgun without credentials",
			cfg:     Config{Env: "prod", MailerProvider: "mailgun"},
			wantErr: true,
		},
		{
			name:    "ses needs a region",
			cfg:     Config{Env: "prod", MailerProvider: "aws_ses", Aws: Aws{Region: "us-east-1"}},
			wantErr: false,
		},
		{
			name:    "ses without a region",
			cfg:     Config{Env: "prod", MailerProvider: "aws_ses"},
			wantErr: true,
		},
		{
			name:    "unknown provider",
			cfg:     Config{Env: "prod", MailerProvider: "carrier-pigeon"},
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
		MailerProvider: "mailgun",
		Mailgun:        Mailgun{ApiKey: "k", Domain: "d"},
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
	unset = append(unset, unsetEnv("MAILER_PROVIDER"))
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
		MailerProvider: "smtp",
		Aws:            Aws{Region: "eu-west-1"},
		Smtp: Smtp{
			SmtpUser:     "user",
			SmtpPassword: "pass",
			SmtpHost:     "host",
			SmtpPort:     123,
			SmtpSecure:   true,
		},
		Mailgun:        Mailgun{ApiKey: "key", Domain: "mg.example.com"},
		DeveloperEmail: "developer@example.com",
	}
	if cfg.GetMailgunApiKey() != "key" {
		t.Error("GetMailgunApiKey failed")
	}
	if cfg.GetMailgunDomain() != "mg.example.com" {
		t.Error("GetMailgunDomain failed")
	}
	if cfg.GetDeveloperEmail() != "developer@example.com" {
		t.Error("GetDeveloperEmail failed")
	}
	if cfg.GetMailerProvider() != "smtp" {
		t.Error("GetMailerProvider failed")
	}
	if cfg.GetAwsRegion() != "eu-west-1" {
		t.Error("GetAwsRegion failed")
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
