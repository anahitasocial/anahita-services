package config

import (
	"anahita-libs/getenv"
	"anahita-libs/settings"
	"errors"
	"os"
	"strings"
)

type Config struct {
	settings.Common

	// The site's address and name, for the links and the heading in an
	// archive.
	PublicWebURL string
	SiteName     string
	// How many days somebody waits between one export and the next, and
	// how many days an archive is kept.
	EveryDays int
	KeepDays  int
}

func NewConfig() *Config {
	config := &Config{
		Common:       settings.ReadCommon(),
		PublicWebURL: strings.TrimRight(os.Getenv("PUBLIC_WEB_URL"), "/"),
		SiteName:     getenv.String("SITE_NAME", "Anahita"),
		EveryDays:    getenv.Int("EXPORT_EVERY_DAYS", 7),
		KeepDays:     getenv.Int("EXPORT_KEEP_DAYS", 7),
	}

	if config.EveryDays < 0 {
		config.EveryDays = 0
	}
	if config.KeepDays < 1 {
		config.KeepDays = 1
	}

	return config
}

func missing(fields map[string]string) error {
	var names []string
	for name, value := range fields {
		if value == "" {
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		return errors.New("missing: " + strings.Join(names, ", "))
	}
	return nil
}

// Validate is what the API needs.
func (config *Config) Validate() error {
	if len(config.SecureCookieKey) != 32 {
		return errors.New("SECURE_COOKIE_KEY must be 32 bytes long")
	}
	return missing(map[string]string{
		"DSN":                config.DSN,
		"FILE_SERVICE_ADDR":  config.FileServiceAddr,
		"GRAPH_SERVICE_ADDR": config.GraphServiceAddr,
		"OAUTH_ISSUER":       config.OAuthIssuer,
		"OAUTH_JWKS_URL":     config.OAuthJWKSURL,
	})
}

// ValidateWorker is what the worker needs, which is less: it answers
// nobody and reads no tokens.
func (config *Config) ValidateWorker() error {
	return missing(map[string]string{
		"DSN":                config.DSN,
		"FILE_SERVICE_ADDR":  config.FileServiceAddr,
		"GRAPH_SERVICE_ADDR": config.GraphServiceAddr,
	})
}

func (config *Config) GetDomainName() string { return os.Getenv("DOMAIN_NAME") }
