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
	ErrMissingFileAddr          = "File gRPC service address is missing"
	ErrMissingGraphAddr         = "Graph gRPC service address is missing"
	ErrMissingAmqpAddr          = "AMQP address is missing"
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
	FileServiceAddr  string
	GraphServiceAddr string
	AmqpAddr         string
	OAuthIssuer      string
	OAuthJWKSURL     string
	DomainName       string
	// Unset, the list built into the service is used. See data/reasons.
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
		AmqpAddr:         os.Getenv("AMQP_ADDR"),
		OAuthIssuer:      os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:     os.Getenv("OAUTH_JWKS_URL"),
		DomainName:       os.Getenv("DOMAIN_NAME"),
	}

	// Derive the JWKS URL from the issuer when not set explicitly.
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

func (config *Config) GetFileServiceAddr() string {
	return config.FileServiceAddr
}

func (config *Config) GetGraphServiceAddr() string {
	return config.GraphServiceAddr
}

func (config *Config) GetAmqpAddr() string {
	return config.AmqpAddr
}

func (config *Config) GetOAuthIssuer() string {
	return config.OAuthIssuer
}

func (config *Config) GetOAuthJWKSURL() string {
	return config.OAuthJWKSURL
}

func (config *Config) GetDomainName() string {
	return config.DomainName
}
