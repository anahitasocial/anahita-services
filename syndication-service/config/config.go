package config

import (
	"anahita-libs/constants"
	"anahita-libs/getenv"
	"errors"
	"log"
	"os"
	"strings"
)

const (
	ErrMissingDSN               = "DSN is missing"
	ErrMissingSecureCookieKey   = "Secure cookie key is missing"
	ErrWrongSizeSecureCookieKey = "Secure cookie key must be 32 bytes long"
	ErrMissingGraphAddr         = "Graph gRPC service address is missing"
	ErrMissingOAuthIssuer       = "OAuth issuer is missing"
	ErrMissingOAuthJWKSURL      = "OAuth JWKS URL is missing"
	ErrInvalidChannelsFrom      = "CHANNELS_FROM must be one of: "
)

// Config is what syndication-service is told from its environment. The API and
// the worker read the same; each checks what it needs.
type Config struct {
	Env              string
	SecureCookieKey  string
	ValkeyAddr       string
	ValkeyPassword   string
	ValkeyDB         int
	FileServiceAddr  string
	GraphServiceAddr string
	OAuthIssuer      string
	OAuthJWKSURL     string
	DomainName       string
	// The database, for the channels, their feeds' items and the domains
	// proved for them.
	DSN string
	// Who may connect a channel. See GetChannelsFrom.
	ChannelsFrom string

	// The worker's. Where link-preview-service is asked for the card of an
	// item's original; unset, articles have no card.
	LinkPreviewURL string
	// The most posts a channel makes in an hour (CHANNEL_FEED_HOURLY_CAP, 10), and
	// how often a feed is read, in minutes (CHANNEL_FEED_READ_EVERY, 30).
	HourlyCap int
	ReadEvery int
}

func NewConfig() *Config {
	config := &Config{
		Env:              getenv.String("ENV", constants.ConfigEnvDev),
		SecureCookieKey:  os.Getenv("SECURE_COOKIE_KEY"),
		ValkeyAddr:       getenv.RenamedString("VALKEY_ADDR", "REDIS_ADDR", "localhost:6379"),
		ValkeyPassword:   getenv.Renamed("VALKEY_PASSWORD", "REDIS_PASSWORD"),
		ValkeyDB:         getenv.RenamedInt("VALKEY_DB", "REDIS_DB", 0),
		FileServiceAddr:  os.Getenv("FILE_SERVICE_ADDR"),
		GraphServiceAddr: os.Getenv("GRAPH_SERVICE_ADDR"),
		OAuthIssuer:      os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:     os.Getenv("OAUTH_JWKS_URL"),
		DomainName:       os.Getenv("DOMAIN_NAME"),
		DSN:              os.Getenv("DSN"),
		// WEBSITES_FROM, WEBSITE_HOURLY_CAP and WEBSITE_READ_EVERY until
		// roadmap row 147, read for one release.
		ChannelsFrom:   getenv.Renamed("CHANNELS_FROM", "WEBSITES_FROM"),
		LinkPreviewURL: os.Getenv("LINK_PREVIEW_URL"),
		HourlyCap:      getenv.RenamedInt("CHANNEL_FEED_HOURLY_CAP", "WEBSITE_HOURLY_CAP", 10),
		ReadEvery:      getenv.RenamedInt("CHANNEL_FEED_READ_EVERY", "WEBSITE_READ_EVERY", 30),
	}

	if config.OAuthJWKSURL == "" && config.OAuthIssuer != "" {
		config.OAuthJWKSURL = strings.TrimRight(config.OAuthIssuer, "/") + "/.well-known/jwks.json"
	}

	return config
}

// Validate is what the API needs.
func (config *Config) Validate() error {
	var missing []string

	if len(config.SecureCookieKey) == 0 {
		missing = append(missing, ErrMissingSecureCookieKey)
	} else if len(config.SecureCookieKey) != 32 {
		missing = append(missing, ErrWrongSizeSecureCookieKey)
	}
	if config.DSN == "" {
		missing = append(missing, ErrMissingDSN)
	}
	if config.GraphServiceAddr == "" {
		missing = append(missing, ErrMissingGraphAddr)
	}
	if config.OAuthIssuer == "" {
		missing = append(missing, ErrMissingOAuthIssuer)
	}
	if config.OAuthJWKSURL == "" {
		missing = append(missing, ErrMissingOAuthJWKSURL)
	}
	if len(missing) > 0 {
		return errors.New("missing fields: " + strings.Join(missing, ", "))
	}

	// A value nobody recognises is a typo, and would refuse every channel with
	// no hint as to why.
	if config.ChannelsFrom != "" && !constants.IsRoleLevel(constants.NormalizeRoleLevel(config.ChannelsFrom)) {
		return errors.New(ErrInvalidChannelsFrom + strings.Join(constants.RoleLevelValues, ", "))
	}

	return nil
}

// ValidateWorker is what the worker needs: the database and the graph.
func (config *Config) ValidateWorker() error {
	var missing []string
	if config.DSN == "" {
		missing = append(missing, ErrMissingDSN)
	}
	if config.GraphServiceAddr == "" {
		missing = append(missing, ErrMissingGraphAddr)
	}
	if len(missing) > 0 {
		return errors.New("missing fields: " + strings.Join(missing, ", "))
	}
	if config.HourlyCap < 1 || config.ReadEvery < 15 {
		return errors.New("CHANNEL_FEED_HOURLY_CAP must be at least 1, and CHANNEL_FEED_READ_EVERY at least 15 minutes")
	}
	return nil
}

func (config *Config) MustValidate() {
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
}

func (config *Config) IsDev() bool {
	return config.Env == constants.ConfigEnvDev
}

func (config *Config) IsStaging() bool {
	return config.Env == constants.ConfigEnvStaging
}

func (config *Config) IsProd() bool {
	return config.Env == constants.ConfigEnvProd
}

func (config *Config) GetSecureCookieKey() string {
	return config.SecureCookieKey
}

func (config *Config) GetValkeyAddr() string {
	return config.ValkeyAddr
}

func (config *Config) GetValkeyPassword() string {
	return config.ValkeyPassword
}

func (config *Config) GetValkeyDB() int {
	return config.ValkeyDB
}

func (config *Config) GetFileServiceAddr() string {
	return config.FileServiceAddr
}

func (config *Config) GetGraphServiceAddr() string {
	return config.GraphServiceAddr
}

func (config *Config) GetOAuthIssuer() string {
	return config.OAuthIssuer
}

func (config *Config) GetOAuthJWKSURL() string {
	return config.OAuthJWKSURL
}

func (config *Config) GetDSN() string {
	return config.DSN
}

func (config *Config) GetDomainName() string {
	return config.DomainName
}

// GetChannelsFrom is the least role that may connect a channel.
//
// Administrators unless an installation says otherwise, as for groups: a
// Channel is a standing profile with a name others will look for, and posts
// to the fediverse in the installation's name.
func (config *Config) GetChannelsFrom() string {
	value := constants.NormalizeRoleLevel(config.ChannelsFrom)
	if value == "" {
		return constants.RoleLevelAdministrators
	}
	return value
}

// MayConnect says whether somebody of this person type meets it. Compared
// by rank, never by equality: see constants.MeetsRoleLevel.
func (config *Config) MayConnect(personType string) bool {
	return constants.MeetsRoleLevel(personType, config.GetChannelsFrom())
}
