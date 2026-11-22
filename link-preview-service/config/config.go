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
	// Unset, the list built into the service is used. See data/reasons.
}

func NewConfig() *Config {
	config := &Config{
		Common:     settings.ReadCommon(),
		DomainName: os.Getenv("DOMAIN_NAME"),
	}

	return config
}

func (config *Config) Validate() error {
	missingFields := config.Missing(settings.CookieKey, settings.FileService, settings.OAuthIssuer,
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
