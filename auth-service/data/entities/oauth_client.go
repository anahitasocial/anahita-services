package entities

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// OAuth grant type constants (RFC 6749 / OIDC).
const (
	GrantTypeAuthorizationCode = "authorization_code"
	GrantTypeRefreshToken      = "refresh_token"
	GrantTypeClientCredentials = "client_credentials"
)

// Default access token lifetime in seconds.
const DefaultOAuthTokenExpiry = 900

// oauthClientIDPattern matches the slug format enforced at the DB level.
var oauthClientIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)

// OAuthClient is an OAuth 2.0 / OIDC client registered with the auth service.
// Clients are first-party today; the FirstParty flag exists for forward
// compatibility with third-party integrations.
type OAuthClient struct {
	ID int64 `db:"id"              json:"id"`

	// ClientID is the public, slug-format identifier used in OAuth protocol
	// messages. Format: ^[a-z0-9][a-z0-9_-]{1,63}$ (enforced at DB level).
	ClientID   string `db:"client_id"   json:"clientId"`
	SecretHash string `db:"secret_hash" json:"-"`
	Name       string `db:"name"        json:"name"`

	RedirectURIs StringSlice `db:"redirect_uris" json:"redirectUris"`
	GrantTypes   StringSlice `db:"grant_types"   json:"grantTypes"`
	Scopes       StringSlice `db:"scopes"        json:"scopes"`

	// TokenExpiry is access token lifetime in seconds.
	TokenExpiry int `db:"token_expiry" json:"tokenExpiry"`

	// Confidential: true for server-side clients with a secret, false for
	// public clients (SPA, native app) that must use PKCE.
	Confidential bool `db:"confidential" json:"confidential"`

	// SkipConsent: when true, the consent screen is bypassed. Only honored
	// for first-party clients; third-party clients always show consent
	// regardless of this flag.
	SkipConsent bool `db:"skip_consent" json:"skipConsent"`

	// FirstParty: true for clients owned by Anahita (the web app, plus
	// any desktop or mobile apps we ship). Third-party clients set this
	// false.
	FirstParty bool `db:"first_party" json:"firstParty"`

	Active    bool      `db:"active"     json:"active"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

// NewOAuthClient returns a new OAuthClient with sensible zero-value defaults.
func NewOAuthClient() *OAuthClient {
	return &OAuthClient{
		ID:           0,
		RedirectURIs: StringSlice{},
		GrantTypes:   StringSlice{},
		Scopes:       StringSlice{},
		TokenExpiry:  DefaultOAuthTokenExpiry,
		Confidential: true,
		SkipConsent:  false,
		FirstParty:   true,
		Active:       true,
	}
}

// Validate checks that the OAuthClient is in a valid state for persistence.
// Returns the first error encountered.
func (c *OAuthClient) Validate() error {
	if !oauthClientIDPattern.MatchString(c.ClientID) {
		return ErrInvalidClientIDFormat
	}
	if strings.TrimSpace(c.Name) == "" {
		return ErrClientNameRequired
	}
	if c.SecretHash == "" && c.Confidential {
		return ErrClientSecretRequired
	}
	if len(c.RedirectURIs) == 0 {
		return ErrRedirectURIRequired
	}
	for _, u := range c.RedirectURIs {
		if !isValidRedirectURI(u) {
			return ErrInvalidRedirectURI
		}
	}
	if len(c.GrantTypes) == 0 {
		return ErrGrantTypeRequired
	}
	for _, gt := range c.GrantTypes {
		if !isKnownGrantType(gt) {
			return ErrInvalidGrantType
		}
	}
	if c.TokenExpiry <= 0 {
		return ErrInvalidTokenExpiry
	}
	return nil
}

// AllowsRedirectURI reports whether the given URI exactly matches one of
// the registered redirect URIs. Exact match only — no prefix matching,
// per OAuth 2.1 guidance.
func (c *OAuthClient) AllowsRedirectURI(uri string) bool {
	return c.RedirectURIs.Contains(uri)
}

// AllowsGrantType reports whether the client may use the given grant type.
func (c *OAuthClient) AllowsGrantType(gt string) bool {
	return c.GrantTypes.Contains(gt)
}

// AllowsScope reports whether the given scope is in the client's declared
// allowable set.
func (c *OAuthClient) AllowsScope(scope string) bool {
	return c.Scopes.Contains(scope)
}

// RequiresConsent reports whether the consent screen must be shown for
// this client. Third-party clients always require consent regardless of
// the SkipConsent flag.
func (c *OAuthClient) RequiresConsent() bool {
	if !c.FirstParty {
		return true
	}
	return !c.SkipConsent
}

// isValidRedirectURI checks that the URI uses an allowed scheme. HTTPS is
// required except for localhost (dev convenience) and custom URI schemes
// used by native/desktop apps.
func isValidRedirectURI(uri string) bool {
	if uri == "" {
		return false
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return false
	}
	switch parsed.Scheme {
	case "https":
		return true
	case "http":
		// HTTP only allowed for loopback hostnames in dev.
		host := parsed.Hostname()
		return host == "localhost" || host == "127.0.0.1" || host == "::1"
	default:
		// Custom scheme for native apps. Must look like reverse-domain
		// (contains at least one dot) to filter out javascript:, data:,
		// vbscript:, file:, about:, etc.
		return strings.Contains(parsed.Scheme, ".")
	}
}

func isKnownGrantType(gt string) bool {
	switch gt {
	case GrantTypeAuthorizationCode, GrantTypeRefreshToken, GrantTypeClientCredentials:
		return true
	}
	return false
}

// Sentinel errors for OAuth client operations.
var (
	ErrClientNotFound        = errors.New("oauth client not found")
	ErrClientInactive        = errors.New("oauth client is inactive")
	ErrInvalidClientIDFormat = errors.New("oauth client_id must match ^[a-z0-9][a-z0-9_-]{1,63}$")
	ErrClientNameRequired    = errors.New("oauth client name is required")
	ErrClientSecretRequired  = errors.New("oauth client secret is required for confidential clients")
	ErrRedirectURIRequired   = errors.New("at least one redirect uri is required")
	ErrInvalidRedirectURI    = errors.New("redirect uri not allowed for client")
	ErrGrantTypeRequired     = errors.New("at least one grant type is required")
	ErrInvalidGrantType      = errors.New("grant type not allowed for client")
	ErrInvalidScope          = errors.New("scope not allowed for client")
	ErrInvalidTokenExpiry    = errors.New("token_expiry must be greater than zero")
	ErrInvalidClientAuth     = errors.New("invalid client credentials")
)
