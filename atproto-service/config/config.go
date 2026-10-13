package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"anahita-libs/constants"
)

// Where Bluesky is read from, unless the installation says otherwise.
const (
	// The public reader of the network: who somebody is, and what they
	// posted. It asks for no account.
	defaultAppView = "https://public.api.bsky.app"
	// Where what is posted is heard as it happens: every one Bluesky runs,
	// listened to in turn, since any one of them can fall behind.
	defaultJetstream = "wss://jetstream1.us-east.bsky.network/subscribe," +
		"wss://jetstream2.us-west.bsky.network/subscribe," +
		"wss://jetstream2.us-east.bsky.network/subscribe," +
		"wss://jetstream1.us-west.bsky.network/subscribe"
	// Where pictures are fetched from, to be copied here.
	defaultCDN = "https://cdn.bsky.app"
)

type Config struct {
	Env              string
	DSN              string
	GraphServiceAddr string
	FileServiceAddr  string
	// The bus the services talk over: where this one hears that somebody
	// here followed somebody on Bluesky.
	AmqpAddr        string
	SecureCookieKey string
	// Where link-preview-service is, for the cards of links in posts from
	// Bluesky. Empty for none.
	LinkPreviewURL string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	OAuthIssuer    string
	OAuthJWKSURL   string

	AppViewURL string
	// One or more, with commas between.
	JetstreamURLs []string
	CDNURL        string
}

func NewConfig() *Config {
	config := &Config{
		Env:              getEnv("ENV", constants.ConfigEnvDev),
		DSN:              os.Getenv("DSN"),
		GraphServiceAddr: os.Getenv("GRAPH_SERVICE_ADDR"),
		FileServiceAddr:  os.Getenv("FILE_SERVICE_ADDR"),
		AmqpAddr:         os.Getenv("AMQP_ADDR"),
		SecureCookieKey:  os.Getenv("SECURE_COOKIE_KEY"),
		LinkPreviewURL:   os.Getenv("LINK_PREVIEW_URL"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		RedisDB:          getIntEnv("REDIS_DB", 0),
		OAuthIssuer:      os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:     os.Getenv("OAUTH_JWKS_URL"),
		AppViewURL:       strings.TrimRight(getEnv("ATPROTO_APPVIEW_URL", defaultAppView), "/"),
		JetstreamURLs:    list(getEnv("ATPROTO_JETSTREAM_URL", defaultJetstream)),
		CDNURL:           strings.TrimRight(getEnv("ATPROTO_CDN_URL", defaultCDN), "/"),
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
	if !strings.HasPrefix(config.AppViewURL, "https://") || !strings.HasPrefix(config.CDNURL, "https://") {
		return errors.New("ATPROTO_APPVIEW_URL and ATPROTO_CDN_URL must be https addresses")
	}
	if len(config.JetstreamURLs) == 0 {
		return errors.New("ATPROTO_JETSTREAM_URL names no address")
	}
	for _, address := range config.JetstreamURLs {
		if !strings.HasPrefix(address, "wss://") {
			return errors.New("ATPROTO_JETSTREAM_URL must be wss addresses, with commas between")
		}
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

// list is what was written with commas between, without what is empty.
func list(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

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
