package config

import (
	"anahita-libs/getenv"
	"anahita-libs/settings"
	"errors"
	"log"
	"strings"
)

// DefaultsPath is where the image keeps them: the dockerfile copies
// feed-defaults to /app/defaults.
const DefaultsPath = "/app/defaults"

type Config struct {
	settings.Common

	DefaultsPath string
}

func NewConfig() *Config {
	config := &Config{
		Common:       settings.ReadCommon(),
		DefaultsPath: getenv.String("DEFAULTS_PATH", DefaultsPath),
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

func (config *Config) GetDefaultsPath() string {
	return config.DefaultsPath
}
