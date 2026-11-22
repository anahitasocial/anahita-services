package config

import (
	"anahita-libs/getenv"
	"anahita-libs/settings"
	"errors"
	"log"
	"strings"
)

const (
	ErrMissingSocialgraphAddr = "Socialgraph gRPC service address is missing"
)

type Config struct {
	settings.Common

	// How many days a read notification is kept after it was read
	// (NOTIFICATIONS_KEEP_DAYS): 90 unless set, 0 for forever. Unread ones
	// are kept until they are read.
	KeepDays int
}

// DefaultKeepDays is how long a read notification is kept when the
// installation does not say.
const DefaultKeepDays = 90

func NewConfig() *Config {
	config := &Config{
		Common:   settings.ReadCommon(),
		KeepDays: getenv.Int("NOTIFICATIONS_KEEP_DAYS", DefaultKeepDays),
	}
	if config.KeepDays < 0 {
		config.KeepDays = DefaultKeepDays
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
