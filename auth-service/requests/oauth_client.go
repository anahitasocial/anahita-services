package requests

import (
	"auth/data/entities"
)

type OAuthClientRequest struct {
	Add    *OAuthClientAddReq
	Read   *OAuthClientReadReq
	Edit   *OAuthClientEditReq
	Delete *OAuthClientDeleteReq
}

func NewOAuthClientRequest() *OAuthClientRequest {
	return &OAuthClientRequest{
		Add:    &OAuthClientAddReq{},
		Read:   &OAuthClientReadReq{},
		Edit:   &OAuthClientEditReq{},
		Delete: &OAuthClientDeleteReq{},
	}
}

// OAuthClientAddReq creates a new OAuth client. ClientID is required and
// must be a slug matching the DB-level CHECK constraint
// (^[a-z0-9][a-z0-9_-]{1,63}$). The handler hashes the generated client
// secret before storing.
type OAuthClientAddReq struct {
	// What it calls itself when it signs people in: lower-case letters, digits, `_` and `-`.
	ClientID string `json:"client_id" validate:"required,min=2,max=64"`
	// What people are shown it is called.
	Name string `json:"name" validate:"required,min=3,max=128"`
	// Where people may be sent back to after signing in.
	RedirectURIs []string `json:"redirect_uris" validate:"required,min=1,dive,url"`
	// How it may get tokens: `authorization_code`, `refresh_token`, `client_credentials`.
	GrantTypes []string `json:"grant_types" validate:"required,min=1,dive,oneof=authorization_code refresh_token client_credentials"`
	// What it may ask to do (oauth.md).
	Scopes entities.StringSlice `json:"scopes" validate:"required"`
	// How long its access tokens last, in seconds: 300 to 86400.
	TokenExpiry int `json:"token_expiry" validate:"omitempty,min=300,max=86400"`
	// Whether it can keep a secret: an app's own server can, an app in a browser or on a phone cannot.
	Confidential bool `json:"confidential"`
	// Whether people are not asked to allow it.
	SkipConsent bool `json:"skip_consent"`
	// Whether it is one of the installation's own apps.
	FirstParty bool `json:"first_party"`
	// Whether it may sign people in.
	Active bool `json:"active"`
}

type OAuthClientReadReq struct {
	// The app's id.
	ID int64 `param:"id" validate:"required"`
}

// OAuthClientEditReq updates a client. ClientID and SecretHash are
// immutable; for credential rotation use the rotate-secret endpoint
// (which triggers token revocation alongside rehashing).
type OAuthClientEditReq struct {
	// The app's id.
	ID int64 `param:"id" validate:"required"`
	// What people are shown it is called.
	Name string `json:"name" validate:"required,min=3,max=128"`
	// Where people may be sent back to after signing in.
	RedirectURIs []string `json:"redirect_uris" validate:"required,min=1,dive,url"`
	// How it may get tokens: `authorization_code`, `refresh_token`, `client_credentials`.
	GrantTypes []string `json:"grant_types" validate:"required,min=1,dive,oneof=authorization_code refresh_token client_credentials"`
	// What it may ask to do (oauth.md).
	Scopes entities.StringSlice `json:"scopes" validate:"required"`
	// How long its access tokens last, in seconds: 300 to 86400.
	TokenExpiry int `json:"token_expiry" validate:"omitempty,min=300,max=86400"`
	// Whether it can keep a secret: an app's own server can, an app in a browser or on a phone cannot.
	Confidential bool `json:"confidential"`
	// Whether people are not asked to allow it.
	SkipConsent bool `json:"skip_consent"`
	// Whether it is one of the installation's own apps.
	FirstParty bool `json:"first_party"`
	// Whether it may sign people in.
	Active bool `json:"active"`
}

type OAuthClientDeleteReq struct {
	// The app's id.
	ID int64 `param:"id" validate:"required"`
}
