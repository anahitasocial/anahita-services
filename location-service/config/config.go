package config

import (
	"anahita-libs/constants"
	"anahita-libs/settings"
	"errors"
	"log"
	"os"
	"strings"
)

const (
	ErrInvalidLocationsFrom = "LOCATIONS_FROM must be one of: "
)

type Config struct {
	settings.Common

	// Who may add a location. See GetLocationsFrom.
	LocationsFrom string
}

func NewConfig() *Config {
	config := &Config{
		Common:        settings.ReadCommon(),
		LocationsFrom: os.Getenv("LOCATIONS_FROM"),
	}

	return config
}

func (config *Config) Validate() error {
	missingFields := config.Missing(settings.CookieKey, settings.FileService,
		settings.GraphService, settings.OAuthIssuer,
		settings.OAuthJWKS)

	if len(missingFields) > 0 {
		return errors.New("missing fields: " + strings.Join(missingFields, ", "))
	}

	if config.LocationsFrom != "" && !constants.IsRoleLevel(constants.NormalizeRoleLevel(config.LocationsFrom)) {
		return errors.New(ErrInvalidLocationsFrom + strings.Join(constants.RoleLevelValues, ", "))
	}

	return nil
}

func (config *Config) MustValidate() {
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
}

// GetLocationsFrom returns the minimum role that may add a location
// (LOCATIONS_FROM): `registered` lets anybody signed in add one,
// `administrators` (the default) keeps it to administrators, as it always
// was. A location is a name others search for and tag their posts with,
// which is why it is kept to administrators until an installation says
// otherwise (the user, 2026-10-18). Editing and deleting one stay with
// administrators whatever this says.
func (config *Config) GetLocationsFrom() string {
	value := constants.NormalizeRoleLevel(config.LocationsFrom)
	if value == "" {
		return constants.RoleLevelAdministrators
	}

	return value
}
