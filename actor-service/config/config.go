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
	ErrMissingSecureCookieKey   = "Secure cookie key is missing"
	ErrWrongSizeSecureCookieKey = "Secure cookie key must be 32 bytes long"
	ErrMissingRedisAddr         = "Redis address is missing"
	ErrMissingAmqpAddr          = "AMQP address is missing"
	ErrMissingFileAddr          = "File gRPC service address is missing"
	ErrMissingGraphAddr         = "Graph gRPC service address is missing"
	ErrMissingOAuthIssuer       = "OAuth issuer is missing"
	ErrMissingOAuthJWKSURL      = "OAuth JWKS URL is missing"
	ErrInvalidGroupsFrom        = "GROUPS_FROM must be one of: "
	ErrInvalidEventsFrom        = "EVENTS_FROM must be one of: "
)

// Config is what actor-service is told from its environment.
//
// There is no database address here, on purpose. The service reads and
// writes profiles through graph-grpc-service and owns no tables, so it is
// given no database credentials to lose.
type Config struct {
	Env              string
	SecureCookieKey  string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	AmqpAddr         string
	FileServiceAddr  string
	GraphServiceAddr string
	OAuthIssuer      string
	OAuthJWKSURL     string
	GroupsFrom       string
	// Who may make an event of their own. See GetEventsFrom.
	EventsFrom string
	// The web app's address, for links that leave the site: the one in a
	// calendar file. Optional; without it the link is left out.
	WebOrigin string
}

func NewConfig() *Config {
	config := &Config{
		Env:              getenv.String("ENV", constants.ConfigEnvDev),
		SecureCookieKey:  os.Getenv("SECURE_COOKIE_KEY"),
		RedisAddr:        getenv.String("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		RedisDB:          getenv.Int("REDIS_DB", 0),
		AmqpAddr:         os.Getenv("AMQP_ADDR"),
		FileServiceAddr:  os.Getenv("FILE_SERVICE_ADDR"),
		GraphServiceAddr: os.Getenv("GRAPH_SERVICE_ADDR"),
		OAuthIssuer:      os.Getenv("OAUTH_ISSUER"),
		OAuthJWKSURL:     os.Getenv("OAUTH_JWKS_URL"),
		GroupsFrom:       os.Getenv("GROUPS_FROM"),
		EventsFrom:       os.Getenv("EVENTS_FROM"),
		WebOrigin:        strings.TrimRight(os.Getenv("WEB_ORIGIN"), "/"),
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

	// Checked against the raw value, not the resolved one: unset is legal and
	// means the default, but a value nobody recognises is a typo, and quietly
	// falling back to administrators would hide it. Unknown levels rank above
	// everything, so booting on one would refuse every group creation with no
	// hint as to why.
	if config.GroupsFrom != "" && !constants.IsRoleLevel(constants.NormalizeRoleLevel(config.GroupsFrom)) {
		return errors.New(ErrInvalidGroupsFrom + strings.Join(constants.RoleLevelValues, ", "))
	}

	// The same for events: a value nobody recognises is a typo, and would
	// refuse every event with no hint as to why.
	if config.EventsFrom != "" && !constants.IsRoleLevel(constants.NormalizeRoleLevel(config.EventsFrom)) {
		return errors.New(ErrInvalidEventsFrom + strings.Join(constants.RoleLevelValues, ", "))
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

// GetWebOrigin returns the web app's address with no slash at its end, or
// "" when it has not been set.
func (config *Config) GetWebOrigin() string {
	return config.WebOrigin
}

// GetGroupsFrom returns the minimum role that may create a group.
//
// Defaults to administrators, which is what happened before this setting
// existed — so no installation changes behaviour by upgrading into it.
func (config *Config) GetGroupsFrom() string {
	value := constants.NormalizeRoleLevel(config.GroupsFrom)
	if value == "" {
		return constants.RoleLevelAdministrators
	}

	return value
}

// GetEventsFrom returns the minimum role that may make an event of their
// own.
//
// Defaults to registered: anybody signed in. A group is a standing place
// with a name others will look for, which is why making one is kept to
// administrators until an installation says otherwise. An event is an
// evening, and an installation where only administrators can hold one would
// have none. One that wants them kept sets EVENTS_FROM.
//
// An event made for a group is not this question: that is for the group's
// administrators, whatever is set here.
func (config *Config) GetEventsFrom() string {
	value := constants.NormalizeRoleLevel(config.EventsFrom)
	if value == "" {
		return constants.RoleLevelRegistered
	}

	return value
}

// MayCreateGroup reports whether somebody of this person type meets the
// configured minimum. Compared by rank, never by equality — see
// constants.MeetsRoleLevel.
func (config *Config) MayCreateGroup(personType string) bool {
	return constants.MeetsRoleLevel(personType, config.GetGroupsFrom())
}
