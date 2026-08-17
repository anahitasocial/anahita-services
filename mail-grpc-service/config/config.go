package config

import (
	an_constants "anahita-libs/constants"
	"errors"
	"log"
	"os"
	"strings"
)

const (
	ErrMissingMailerProvider = "Mailer provider is missing"
	ErrMissingSmtpUser       = "SMTP user is missing"
	ErrMissingSmtpPassword   = "SMTP password is missing"
	ErrMissingSmtpHost       = "SMTP host is missing"
	ErrMissingSmtpPort       = "SMTP port is missing"
)

type Smtp struct {
	SmtpUser     string
	SmtpPassword string
	SmtpHost     string
	SmtpPort     int
	SmtpSecure   bool
}

type Aws struct {
	Region string
}

type Config struct {
	MailerProvider string
	Aws            Aws
	Smtp           Smtp
	Env            string
}

func NewConfig() *Config {
	config := &Config{
		Env:            getEnv("ENV", an_constants.ConfigEnvDev),
		MailerProvider: os.Getenv("MAILER_PROVIDER"),
		Aws: Aws{
			Region: os.Getenv("AWS_REGION"),
		},
		Smtp: Smtp{
			SmtpUser:     os.Getenv("SMTP_USER"),
			SmtpPassword: os.Getenv("SMTP_PASSWORD"),
			SmtpHost:     os.Getenv("SMTP_HOST"),
			SmtpPort:     getIntEnv("SMTP_PORT", 587),
			SmtpSecure:   getBoolEnv("SMTP_SECURE", false),
		},
	}

	return config
}

func (config *Config) Validate() error {
	var missingFields []string

	if config.MailerProvider == "" {
		missingFields = append(missingFields, ErrMissingMailerProvider)
	}

	if config.Smtp.SmtpUser == "" {
		missingFields = append(missingFields, ErrMissingSmtpUser)
	}

	if config.Smtp.SmtpPassword == "" {
		missingFields = append(missingFields, ErrMissingSmtpPassword)
	}

	if config.Smtp.SmtpHost == "" {
		missingFields = append(missingFields, ErrMissingSmtpHost)
	}

	if config.Smtp.SmtpPort == 0 {
		missingFields = append(missingFields, ErrMissingSmtpPort)
	}

	if len(missingFields) > 0 {
		return errors.New("missing fields: " + strings.Join(missingFields, ", "))
	}

	return nil
}

func (config *Config) MustValidate() {
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
}

func (config *Config) IsDev() bool {
	return config.Env == an_constants.ConfigEnvDev
}

func (config *Config) IsStaging() bool {
	return config.Env == an_constants.ConfigEnvStaging
}

func (config *Config) IsProd() bool {
	return config.Env == an_constants.ConfigEnvProd
}

// GetMailerProvider returns the mailer provider
func (config *Config) GetMailerProvider() string {
	return config.MailerProvider
}

// GetAwsRegion returns the AWS region
func (config *Config) GetAwsRegion() string {
	return config.Aws.Region
}

// GetSmtpUser returns the SMTP user
func (config *Config) GetSmtpUser() string {
	return config.Smtp.SmtpUser
}

// GetSmtpPassword returns the SMTP password
func (config *Config) GetSmtpPassword() string {
	return config.Smtp.SmtpPassword
}

// GetSmtpHost returns the SMTP host
func (config *Config) GetSmtpHost() string {
	return config.Smtp.SmtpHost
}

// GetSmtpPort returns the SMTP port
func (config *Config) GetSmtpPort() int {
	return config.Smtp.SmtpPort
}

// GetSmtpSecure returns whether SMTP is secure
func (config *Config) GetSmtpSecure() bool {
	return config.Smtp.SmtpSecure
}
