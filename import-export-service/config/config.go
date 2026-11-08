package config

import (
	"anahita-libs/getenv"
	"errors"
	"os"
	"strings"

	"anahita-libs/constants"
)

type Config struct {
	Env              string
	DSN              string
	SecureCookieKey  string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	FileServiceAddr  string
	GraphServiceAddr string
	OAuthIssuer      string
	OAuthJWKSURL     string
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
		Env:              getenv.String("ENV", constants.ConfigEnvDev),
		DSN:              os.Getenv("DSN"),
		SecureCookieKey:  os.Getenv("SECURE_COOKIE_KEY"),
		RedisAddr:        getenv.String("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		RedisDB:          getenv.Int("REDIS_DB", 0),
		FileServiceAddr:  os.Getenv("FILE_SERVICE_ADDR"),
		GraphServiceAddr: os.Getenv("GRAPH_SERVICE_ADDR"),
		OAuthIssuer:      os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:     os.Getenv("OAUTH_JWKS_URL"),
		PublicWebURL:     strings.TrimRight(os.Getenv("PUBLIC_WEB_URL"), "/"),
		SiteName:         getenv.String("SITE_NAME", "Anahita"),
		EveryDays:        getenv.Int("EXPORT_EVERY_DAYS", 7),
		KeepDays:         getenv.Int("EXPORT_KEEP_DAYS", 7),
	}

	if config.OAuthJWKSURL == "" && config.OAuthIssuer != "" {
		config.OAuthJWKSURL = strings.TrimRight(config.OAuthIssuer, "/") + "/.well-known/jwks.json"
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

func (config *Config) IsDev() bool     { return config.Env == constants.ConfigEnvDev }
func (config *Config) IsStaging() bool { return config.Env == constants.ConfigEnvStaging }
func (config *Config) IsProd() bool    { return config.Env == constants.ConfigEnvProd }

func (config *Config) GetDSN() string              { return config.DSN }
func (config *Config) GetSecureCookieKey() string  { return config.SecureCookieKey }
func (config *Config) GetRedisAddr() string        { return config.RedisAddr }
func (config *Config) GetRedisPassword() string    { return config.RedisPassword }
func (config *Config) GetRedisDB() int             { return config.RedisDB }
func (config *Config) GetFileServiceAddr() string  { return config.FileServiceAddr }
func (config *Config) GetGraphServiceAddr() string { return config.GraphServiceAddr }
func (config *Config) GetOAuthIssuer() string      { return config.OAuthIssuer }
func (config *Config) GetOAuthJWKSURL() string     { return config.OAuthJWKSURL }
func (config *Config) GetDomainName() string       { return os.Getenv("DOMAIN_NAME") }
