package config

import (
	"anahita-libs/constants"
	"anahita-libs/videos"
	"errors"
	"log"
	"os"
	"strconv"
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
	// The database, for the uploads this service keeps.
	DSN string
	// The largest video taken, in bytes, and the longest, in seconds.
	VideoMaxBytes   int64
	VideoMaxSeconds int
	// How many videos one person may send in a day.
	VideoDailyUploads int
}

func NewConfig() *Config {
	config := &Config{
		Env:               getEnv("ENV", constants.ConfigEnvDev),
		SecureCookieKey:   os.Getenv("SECURE_COOKIE_KEY"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     os.Getenv("REDIS_PASSWORD"),
		RedisDB:           getIntEnv("REDIS_DB", 0),
		FileServiceAddr:   os.Getenv("FILE_SERVICE_ADDR"),
		GraphServiceAddr:  os.Getenv("GRAPH_SERVICE_ADDR"),
		OAuthIssuer:       os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:      os.Getenv("OAUTH_JWKS_URL"),
		DSN:               os.Getenv("DSN"),
		VideoMaxBytes:     videos.MaxBytes(),
		VideoMaxSeconds:   videos.MaxSeconds(),
		VideoDailyUploads: getIntEnv("VIDEO_DAILY_UPLOADS", 20),
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
		missingFields = append(missingFields, "DSN is missing")
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

func (config *Config) GetDSN() string {
	return config.DSN
}

// ValidateWorker is what the worker needs, which is less than the API: it
// answers nobody and reads no tokens.
func (config *Config) ValidateWorker() error {
	var missing []string

	if config.DSN == "" {
		missing = append(missing, "DSN is missing")
	}
	if config.FileServiceAddr == "" {
		missing = append(missing, ErrMissingFileAddr)
	}
	if config.GraphServiceAddr == "" {
		missing = append(missing, ErrMissingGraphAddr)
	}
	if len(missing) > 0 {
		return errors.New("missing fields: " + strings.Join(missing, ", "))
	}
	return nil
}

// Heights reads VIDEO_RENDITIONS, "1080,720,360": the sizes a video is made
// in, by height, up to 2160 (4K). A video is never made larger than it is.
func Heights() []int {
	var heights []int
	for _, part := range strings.Split(getEnv("VIDEO_RENDITIONS", "1080,720,360"), ",") {
		height, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && height >= 144 && height <= 2160 {
			heights = append(heights, height)
		}
	}
	if len(heights) == 0 {
		heights = []int{1080, 720, 360}
	}
	return heights
}

// GetDomainName is the site's domain, which the session store is built
// with. Only its counter is used here, to limit downloads.
func (config *Config) GetDomainName() string {
	return os.Getenv("DOMAIN_NAME")
}
