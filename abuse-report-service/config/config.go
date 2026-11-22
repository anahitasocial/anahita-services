package config

import (
	"anahita-libs/settings"
	"errors"
	"log"
	"os"
	"strings"
)

type Config struct {
	settings.Common

	DomainName string

	// ReasonsPath is an installation's own list of reasons for reporting.
	// Unset, the list built into the service is used. See data/reasons.
	ReasonsPath string
}

func NewConfig() *Config {
	config := &Config{
		Common:      settings.ReadCommon(),
		DomainName:  os.Getenv("DOMAIN_NAME"),
		ReasonsPath: os.Getenv("REASONS_PATH"),
	}

	return config
}

func (config *Config) Validate() error {
	missingFields := config.Missing(settings.Database, settings.CookieKey, settings.FileService,
		settings.GraphService, settings.Amqp, settings.OAuthIssuer,
		settings.OAuthJWKS)

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

func (config *Config) GetDomainName() string {
	return config.DomainName
}

func (config *Config) GetReasonsPath() string {
	return config.ReasonsPath
}
