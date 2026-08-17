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
