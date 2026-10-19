package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"anahita-libs/constants"
)

type Config struct {
	Env              string
	DSN              string
	GraphServiceAddr string
	FileServiceAddr  string
	// The bus the services talk over: where this one hears that somebody
	// here followed somebody elsewhere.
	AmqpAddr        string
	SecureCookieKey string
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
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	OAuthIssuer    string
	OAuthJWKSURL   string
	// How the private half of each person's signing key is kept: encrypted,
	// by the same means auth-service keeps its secrets.
	CryptBackend  string
	CryptLocalKey string
	AwsKmsKeyId   string
}

func NewConfig() *Config {
	config := &Config{
		Env:              getEnv("ENV", constants.ConfigEnvDev),
		DSN:              os.Getenv("DSN"),
		GraphServiceAddr: os.Getenv("GRAPH_SERVICE_ADDR"),
		FileServiceAddr:  os.Getenv("FILE_SERVICE_ADDR"),
		AmqpAddr:         os.Getenv("AMQP_ADDR"),
		SecureCookieKey:  os.Getenv("SECURE_COOKIE_KEY"),
		Lab:              os.Getenv("ACTIVITYPUB_LAB") == "on",
		LinkPreviewURL:   os.Getenv("LINK_PREVIEW_URL"),
		TakesLocations:   strings.ToLower(strings.TrimSpace(os.Getenv("FEDERATION_LOCATIONS"))) != "off",
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		RedisDB:          getIntEnv("REDIS_DB", 0),
		OAuthIssuer:      os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:     os.Getenv("OAUTH_JWKS_URL"),
		CryptBackend:     getEnv("CRYPT_BACKEND", constants.CryptBackendLocal),
		CryptLocalKey:    os.Getenv("CRYPT_LOCAL_KEY"),
		AwsKmsKeyId:      os.Getenv("AWS_KMS_KEY_ID"),
	}

	if config.OAuthJWKSURL == "" && config.OAuthIssuer != "" {
		config.OAuthJWKSURL = strings.TrimRight(config.OAuthIssuer, "/") + "/.well-known/jwks.json"
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

func (config *Config) IsDev() bool     { return config.Env == constants.ConfigEnvDev }
func (config *Config) IsStaging() bool { return config.Env == constants.ConfigEnvStaging }
func (config *Config) IsProd() bool    { return config.Env == constants.ConfigEnvProd }

func (config *Config) GetDSN() string              { return config.DSN }
func (config *Config) GetGraphServiceAddr() string { return config.GraphServiceAddr }
func (config *Config) GetFileServiceAddr() string  { return config.FileServiceAddr }
func (config *Config) GetAmqpAddr() string         { return config.AmqpAddr }
func (config *Config) GetSecureCookieKey() string  { return config.SecureCookieKey }
func (config *Config) GetRedisAddr() string        { return config.RedisAddr }
func (config *Config) GetRedisPassword() string    { return config.RedisPassword }
func (config *Config) GetRedisDB() int             { return config.RedisDB }
func (config *Config) GetOAuthIssuer() string      { return config.OAuthIssuer }
func (config *Config) GetOAuthJWKSURL() string     { return config.OAuthJWKSURL }
func (config *Config) GetDomainName() string       { return os.Getenv("DOMAIN_NAME") }
func (config *Config) GetCryptBackend() string     { return config.CryptBackend }
func (config *Config) GetCryptLocalKey() string    { return config.CryptLocalKey }
func (config *Config) GetAwsKmsKeyId() string      { return config.AwsKmsKeyId }

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if value, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return value
	}
	return fallback
}
