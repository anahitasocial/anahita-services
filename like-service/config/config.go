package config

import (
	"anahita-libs/settings"
	"errors"
	"log"
	"strings"
)

type Config struct {
	settings.Common
}

func NewConfig() *Config {
	config := &Config{
		Common: settings.ReadCommon(),
	}

	return config
}

func (config *Config) Validate() error {
	missingFields := config.Missing(settings.CookieKey, settings.Database, settings.Amqp,
		settings.FileService, settings.GraphService,
		settings.OAuthIssuer, settings.OAuthJWKS)

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
