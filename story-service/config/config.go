package config

import (
	"anahita-libs/constants"
	"errors"
	"log"
	"os"
	"strings"
)

const (
	ErrMissingDSN               = "DSN is missing"
	ErrMissingSecureCookieKey   = "Secure cookie key is missing"
	ErrWrongSizeSecureCookieKey = "Secure cookie key must be 32 bytes long"
	ErrMissingRedisAddr         = "Redis address is missing"
	ErrMissingAmqpAddr          = "AMQP address is missing"
	ErrMissingFileAddr          = "File gRPC service address is missing"
	ErrMissingGraphAddr         = "Graph gRPC service address is missing"
	ErrMissingOAuthIssuer       = "OAuth issuer is missing"
	ErrMissingOAuthJWKSURL      = "OAuth JWKS URL is missing"
)

type Config struct {
	Env              string
	DSN              string
	SecureCookieKey  string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	AmqpAddr         string
	FileServiceAddr  string
	GraphServiceAddr string
	OAuthIssuer      string
	OAuthJWKSURL     string
}

func NewConfig() *Config {
	config := &Config{
		Env:              getEnv("ENV", constants.ConfigEnvDev),
		DSN:              os.Getenv("DSN"),
		SecureCookieKey:  os.Getenv("SECURE_COOKIE_KEY"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		RedisDB:          getIntEnv("REDIS_DB", 0),
		AmqpAddr:         os.Getenv("AMQP_ADDR"),
		FileServiceAddr:  os.Getenv("FILE_SERVICE_ADDR"),
		GraphServiceAddr: os.Getenv("GRAPH_SERVICE_ADDR"),
		OAuthIssuer:      os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:     os.Getenv("OAUTH_JWKS_URL"),
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

	if config.DSN == "" {
		missingFields = append(missingFields, ErrMissingDSN)
	}

	if config.AmqpAddr == "" {
		missingFields = append(missingFields, ErrMissingAmqpAddr)
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

func (config *Config) GetDSN() string {
	return config.DSN
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

func (config *Config) GetAmqpAddr() string {
	return config.AmqpAddr
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
