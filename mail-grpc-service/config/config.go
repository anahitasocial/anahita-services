package config

import (
	an_constants "anahita-libs/constants"
	"errors"
	"log"
	"os"
	"strings"

	"mail/constants"
)

const (
	ErrMissingMailerProvider = "Mailer provider is missing"
	ErrMissingSmtpUser       = "SMTP user is missing"
	ErrMissingSmtpPassword   = "SMTP password is missing"
	ErrMissingSmtpHost       = "SMTP host is missing"
	ErrMissingSmtpPort       = "SMTP port is missing"
	ErrMissingAwsRegion      = "AWS region is missing"
	ErrMissingMailgunApiKey  = "Mailgun API key is missing"
	ErrMissingMailgunDomain  = "Mailgun domain is missing"
	ErrMissingDeveloperEmail = "Developer email is missing, and is required in development"
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

// Mailgun credentials.
//
// These used to be read off the Smtp struct — NewMailGunMailer took SmtpHost as
// the domain and SmtpPassword as the API key. That worked only by coincidence
// and made the SMTP fields required for a deployment that does not use SMTP.
type Mailgun struct {
	ApiKey string
	Domain string
}

type Config struct {
	MailerProvider string
	Aws            Aws
	Smtp           Smtp
	Mailgun        Mailgun
	Env            string

	// DeveloperEmail is where every outbound message goes in development.
	//
	// Without it, testing any flow that sends mail means either sending real
	// mail to real addresses from a laptop, or not testing it. Several flows in
	// auth-service — password reset, email change, account deletion — cannot be
	// exercised at all without receiving the message, since the link in it is
	// the next step.
	//
	// Required only in development; see Validate.
	DeveloperEmail string
}

func NewConfig() *Config {
	config := &Config{
		Env:            getEnv("ENV", an_constants.ConfigEnvDev),
		MailerProvider: os.Getenv("MAILER_PROVIDER"),
		DeveloperEmail: os.Getenv("DEVELOPER_EMAIL"),
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
		Mailgun: Mailgun{
			ApiKey: os.Getenv("MAILGUN_API_KEY"),
			Domain: os.Getenv("MAILGUN_DOMAIN"),
		},
	}

	return config
}

// Validate checks only the configuration the selected provider actually uses.
//
// It used to require SMTP_USER, SMTP_PASSWORD, SMTP_HOST and SMTP_PORT
// unconditionally, so a deployment running Mailgun or SES could not start
// without inventing SMTP credentials it would never use.
func (config *Config) Validate() error {
	var missingFields []string

	switch config.MailerProvider {
	case constants.MailerProviderSmtp:
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
	case constants.MailerProviderAwsSes:
		if config.Aws.Region == "" {
			missingFields = append(missingFields, ErrMissingAwsRegion)
		}
	case constants.MailerProviderMailgun:
		if config.Mailgun.ApiKey == "" {
			missingFields = append(missingFields, ErrMissingMailgunApiKey)
		}
		if config.Mailgun.Domain == "" {
			missingFields = append(missingFields, ErrMissingMailgunDomain)
		}
	default:
		missingFields = append(missingFields, ErrMissingMailerProvider)
	}

	// Required in development only.
	//
	// The redirect is what stops a laptop mailing real people, so booting
	// without it in development is the one case worth refusing outright — an
	// empty override would silently blank the recipient list instead. Requiring
	// it everywhere, as protect-services does, only forces production
	// deployments to carry a value nothing reads.
	if config.IsDev() && config.DeveloperEmail == "" {
		missingFields = append(missingFields, ErrMissingDeveloperEmail)
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

// GetMailgunApiKey returns the Mailgun API key
func (config *Config) GetMailgunApiKey() string {
	return config.Mailgun.ApiKey
}

// GetMailgunDomain returns the Mailgun sending domain
func (config *Config) GetMailgunDomain() string {
	return config.Mailgun.Domain
}

// GetDeveloperEmail returns the address every message is redirected to in
// development. Empty outside development, where nothing reads it.
func (config *Config) GetDeveloperEmail() string {
	return config.DeveloperEmail
}
