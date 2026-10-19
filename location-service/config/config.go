package config

import (
	"anahita-libs/constants"
	"errors"
	"log"
	"os"
	"strings"
)

const (
	ErrMissingSecureCookieKey   = "Secure cookie key is missing"
	ErrWrongSizeSecureCookieKey = "Secure cookie key must be 32 bytes long"
	ErrMissingRedisAddr         = "Redis address is missing"
	ErrMissingFileAddr          = "File gRPC service address is missing"
	ErrMissingGraphAddr         = "Graph gRPC service address is missing"
	ErrMissingOAuthIssuer       = "OAuth issuer is missing"
	ErrMissingOAuthJWKSURL      = "OAuth JWKS URL is missing"
	ErrInvalidLocationsFrom     = "LOCATIONS_FROM must be one of: "
)

type Config struct {
	Env              string
	SecureCookieKey  string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	FileServiceAddr  string
	GraphServiceAddr string
	OAuthIssuer      string
	OAuthJWKSURL     string
	// Who may add a location. See GetLocationsFrom.
	LocationsFrom string
}

func NewConfig() *Config {
	config := &Config{
		Env:              getEnv("ENV", constants.ConfigEnvDev),
		SecureCookieKey:  os.Getenv("SECURE_COOKIE_KEY"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		RedisDB:          getIntEnv("REDIS_DB", 0),
		FileServiceAddr:  os.Getenv("FILE_SERVICE_ADDR"),
		GraphServiceAddr: os.Getenv("GRAPH_SERVICE_ADDR"),
		OAuthIssuer:      os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:     os.Getenv("OAUTH_JWKS_URL"),
		LocationsFrom:    os.Getenv("LOCATIONS_FROM"),
	}

	// If JWKS URL is unset, derive it from the issuer — auth-service is
	// its own JWKS publisher in the standard OIDC layout.
	if config.OAuthJWKSURL == "" && config.OAuthIssuer != "" {
		config.OAuthJWKSURL = strings.TrimRight(config.OAuthIssuer, "/") + "/.well-known/jwks.json"
	}

	return config
}

func (config *Config) Validate() error {
	var missingFields []string

	if len(config.SecureCookieKey) == 0 {
		missingFields = append(missingFields, ErrMissingSecureCookieKey)
	}

	if len(config.SecureCookieKey) != 32 {
		missingFields = append(missingFields, ErrWrongSizeSecureCookieKey)
	}

	if config.FileServiceAddr == "" {
		missingFields = append(missingFields, ErrMissingFileAddr)
	}

	if config.GraphServiceAddr == "" {
		missingFields = append(missingFields, ErrMissingGraphAddr)
	}

	if config.OAuthIssuer == "" {
		missingFields = append(missingFields, ErrMissingOAuthIssuer)
	}

	if config.OAuthJWKSURL == "" {
		missingFields = append(missingFields, ErrMissingOAuthJWKSURL)
	}

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

func (config *Config) GetRedisAddr() string {
	return config.RedisAddr
}

func (config *Config) GetRedisPassword() string {
	return config.RedisPassword
}

func (config *Config) GetRedisDB() int {
	return config.RedisDB
}

func (config *Config) GetFileServiceAddr() string {
	return config.FileServiceAddr
}

func (config *Config) GetGraphServiceAddr() string {
	return config.GraphServiceAddr
}

// GetOAuthIssuer returns the canonical issuer URL of auth-service.
// Tokens whose "iss" claim does not match are rejected. Must be
// identical across every service sharing an auth-service deployment.
func (config *Config) GetOAuthIssuer() string {
	return config.OAuthIssuer
}

// GetOAuthJWKSURL returns the URL of auth-service's JWKS endpoint. The
// anahita-libs OAuth middleware fetches public keys from here to verify
// JWT signatures locally.
func (config *Config) GetOAuthJWKSURL() string {
	return config.OAuthJWKSURL
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
