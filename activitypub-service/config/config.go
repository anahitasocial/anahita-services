package config

import (
	"anahita-libs/getenv"
	"anahita-libs/settings"
	"errors"
	"os"
	"strings"

	"anahita-libs/constants"
)

type Config struct {
	settings.Common

	// For the federation lab only: go to private addresses, where the
	// lab's servers are.
	Lab bool
	// Where link-preview-service is, for the cards of links in posts from
	// other servers. Empty for none.
	LinkPreviewURL string
	// Whether a post from elsewhere is tagged with the locations it names
	// (FEDERATION_LOCATIONS): on unless it says `off` (the user, 2026-10-19;
	// roadmap row 82, part 4).
	TakesLocations bool
	// How the private half of each person's signing key is kept: encrypted,
	// by the same means auth-service keeps its secrets.
	CryptBackend  string
	CryptLocalKey string
	AwsKmsKeyId   string
}

func NewConfig() *Config {
	config := &Config{
		Common:         settings.ReadCommon(),
		Lab:            os.Getenv("ACTIVITYPUB_LAB") == "on",
		LinkPreviewURL: os.Getenv("LINK_PREVIEW_URL"),
		TakesLocations: strings.ToLower(strings.TrimSpace(os.Getenv("FEDERATION_LOCATIONS"))) != "off",
		CryptBackend:   getenv.String("CRYPT_BACKEND", constants.CryptBackendLocal),
		CryptLocalKey:  os.Getenv("CRYPT_LOCAL_KEY"),
		AwsKmsKeyId:    os.Getenv("AWS_KMS_KEY_ID"),
	}

	return config
}

func (config *Config) Validate() error {
	if len(config.SecureCookieKey) != 32 {
		return errors.New("SECURE_COOKIE_KEY must be 32 bytes long")
	}
	// The lab's rule lets this service reach inside the network it is on.
	// Never on a live site.
	if config.Lab && config.IsProd() {
		return errors.New("ACTIVITYPUB_LAB is for the federation lab and cannot be on with ENV=prod")
	}
	var names []string
	for name, value := range map[string]string{
		"DSN":                config.DSN,
		"GRAPH_SERVICE_ADDR": config.GraphServiceAddr,
		"FILE_SERVICE_ADDR":  config.FileServiceAddr,
		"AMQP_ADDR":          config.AmqpAddr,
		"OAUTH_ISSUER":       config.OAuthIssuer,
		"OAUTH_JWKS_URL":     config.OAuthJWKSURL,
	} {
		if value == "" {
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		return errors.New("missing: " + strings.Join(names, ", "))
	}
	return nil
}

func (config *Config) GetDomainName() string    { return os.Getenv("DOMAIN_NAME") }
func (config *Config) GetCryptBackend() string  { return config.CryptBackend }
func (config *Config) GetCryptLocalKey() string { return config.CryptLocalKey }
func (config *Config) GetAwsKmsKeyId() string   { return config.AwsKmsKeyId }
