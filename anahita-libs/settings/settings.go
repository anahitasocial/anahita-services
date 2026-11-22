// Package settings is what every service reads from the environment alike:
// where it runs, its cookie key, Valkey, the database, the event bus, the
// graph and file services, and the issuer of sign-in tokens (roadmap row
// 170: each service's configuration kept its own copy, about 2,000 lines
// in all). A service's Config embeds Common and adds its own, and keeps its
// own Validate, which says which of these it needs.
package settings

import (
	"os"
	"strings"

	"anahita-libs/constants"
	"anahita-libs/getenv"
)

// Common is the settings every service reads alike.
type Common struct {
	// Env is where it runs: `dev`, `staging` or `prod` (ENV).
	Env string
	// SecureCookieKey signs and encrypts cookies: 32 bytes
	// (SECURE_COOKIE_KEY).
	SecureCookieKey string
	// Valkey (VALKEY_ADDR, VALKEY_PASSWORD, VALKEY_DB; the REDIS_ names
	// before them are read too).
	ValkeyAddr     string
	ValkeyPassword string
	ValkeyDB       int
	// The services it calls (FILE_SERVICE_ADDR, GRAPH_SERVICE_ADDR).
	FileServiceAddr  string
	GraphServiceAddr string
	// AmqpAddr is the event bus, RabbitMQ (AMQP_ADDR).
	AmqpAddr string
	// DSN is its MySQL database (DSN).
	DSN string
	// OAuthIssuer is auth-service's address, which every sign-in token
	// names; a token that names another is refused (OAUTH_ISSUER). The
	// same in every service of an installation.
	OAuthIssuer string
	// OAuthJWKSURL is where auth-service publishes the keys tokens are
	// checked with (OAUTH_JWKS_URL): the issuer's own, by default.
	OAuthJWKSURL string
}

// ReadCommon reads the common settings from the environment.
func ReadCommon() Common {
	common := Common{
		Env:              getenv.String("ENV", constants.ConfigEnvDev),
		SecureCookieKey:  os.Getenv("SECURE_COOKIE_KEY"),
		ValkeyAddr:       getenv.RenamedString("VALKEY_ADDR", "REDIS_ADDR", "localhost:6379"),
		ValkeyPassword:   getenv.Renamed("VALKEY_PASSWORD", "REDIS_PASSWORD"),
		ValkeyDB:         getenv.RenamedInt("VALKEY_DB", "REDIS_DB", 0),
		FileServiceAddr:  os.Getenv("FILE_SERVICE_ADDR"),
		GraphServiceAddr: os.Getenv("GRAPH_SERVICE_ADDR"),
		AmqpAddr:         os.Getenv("AMQP_ADDR"),
		DSN:              os.Getenv("DSN"),
		OAuthIssuer:      os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:     os.Getenv("OAUTH_JWKS_URL"),
	}
	// Unset, the keys are the issuer's own: auth-service publishes them,
	// as an OpenID provider does.
	if common.OAuthJWKSURL == "" && common.OAuthIssuer != "" {
		common.OAuthJWKSURL = strings.TrimRight(common.OAuthIssuer, "/") + "/.well-known/jwks.json"
	}
	return common
}

func (c Common) IsDev() bool     { return c.Env == constants.ConfigEnvDev }
func (c Common) IsStaging() bool { return c.Env == constants.ConfigEnvStaging }
func (c Common) IsProd() bool    { return c.Env == constants.ConfigEnvProd }

func (c Common) GetSecureCookieKey() string  { return c.SecureCookieKey }
func (c Common) GetValkeyAddr() string       { return c.ValkeyAddr }
func (c Common) GetValkeyPassword() string   { return c.ValkeyPassword }
func (c Common) GetValkeyDB() int            { return c.ValkeyDB }
func (c Common) GetFileServiceAddr() string  { return c.FileServiceAddr }
func (c Common) GetGraphServiceAddr() string { return c.GraphServiceAddr }
func (c Common) GetAmqpAddr() string         { return c.AmqpAddr }
func (c Common) GetDSN() string              { return c.DSN }
func (c Common) GetOAuthIssuer() string      { return c.OAuthIssuer }
func (c Common) GetOAuthJWKSURL() string     { return c.OAuthJWKSURL }

// What is said of a common setting that is missing, or wrong.
const (
	ErrMissingSecureCookieKey   = "Secure cookie key is missing"
	ErrWrongSizeSecureCookieKey = "Secure cookie key must be 32 bytes long"
	ErrMissingValkeyAddr        = "Valkey address is missing"
	ErrMissingFileAddr          = "File gRPC service address is missing"
	ErrMissingGraphAddr         = "Graph gRPC service address is missing"
	ErrMissingAmqpAddr          = "AMQP address is missing"
	ErrMissingDSN               = "DSN is missing"
	ErrMissingOAuthIssuer       = "OAuth issuer is missing"
	ErrMissingOAuthJWKSURL      = "OAuth JWKS URL is missing"
)

// Need is a common setting a service cannot run without.
type Need int

const (
	CookieKey Need = iota
	Valkey
	FileService
	GraphService
	Amqp
	Database
	OAuthIssuer
	OAuthJWKS
)

// Missing is what is wrong with the common settings a service needs, said
// in the order they are asked for: empty when nothing is. An empty cookie
// key is missing, and said once, not also the wrong size.
func (c Common) Missing(needs ...Need) []string {
	var missing []string
	for _, need := range needs {
		switch {
		case need == CookieKey && c.SecureCookieKey == "":
			missing = append(missing, ErrMissingSecureCookieKey)
		case need == CookieKey && len(c.SecureCookieKey) != 32:
			missing = append(missing, ErrWrongSizeSecureCookieKey)
		case need == Valkey && c.ValkeyAddr == "":
			missing = append(missing, ErrMissingValkeyAddr)
		case need == FileService && c.FileServiceAddr == "":
			missing = append(missing, ErrMissingFileAddr)
		case need == GraphService && c.GraphServiceAddr == "":
			missing = append(missing, ErrMissingGraphAddr)
		case need == Amqp && c.AmqpAddr == "":
			missing = append(missing, ErrMissingAmqpAddr)
		case need == Database && c.DSN == "":
			missing = append(missing, ErrMissingDSN)
		case need == OAuthIssuer && c.OAuthIssuer == "":
			missing = append(missing, ErrMissingOAuthIssuer)
		case need == OAuthJWKS && c.OAuthJWKSURL == "":
			missing = append(missing, ErrMissingOAuthJWKSURL)
		}
	}
	return missing
}
