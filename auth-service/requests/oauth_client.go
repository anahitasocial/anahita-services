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
	ClientID     string               `json:"client_id" validate:"required,min=2,max=64"`
	Name         string               `json:"name" validate:"required,min=3,max=128"`
	RedirectURIs []string             `json:"redirect_uris" validate:"required,min=1,dive,url"`
	GrantTypes   []string             `json:"grant_types" validate:"required,min=1,dive,oneof=authorization_code refresh_token client_credentials"`
	Scopes       entities.StringSlice `json:"scopes" validate:"required"`
	TokenExpiry  int                  `json:"token_expiry" validate:"omitempty,min=300,max=86400"`
	Confidential bool                 `json:"confidential"`
	SkipConsent  bool                 `json:"skip_consent"`
	FirstParty   bool                 `json:"first_party"`
	Active       bool                 `json:"active"`
}

type OAuthClientReadReq struct {
	ID int64 `param:"id" validate:"required"`
}

// OAuthClientEditReq updates a client. ClientID and SecretHash are
// immutable; for credential rotation use the rotate-secret endpoint
// (which triggers token revocation alongside rehashing).
type OAuthClientEditReq struct {
	ID           int64                `param:"id" validate:"required"`
	Name         string               `json:"name" validate:"required,min=3,max=128"`
	RedirectURIs []string             `json:"redirect_uris" validate:"required,min=1,dive,url"`
	GrantTypes   []string             `json:"grant_types" validate:"required,min=1,dive,oneof=authorization_code refresh_token client_credentials"`
	Scopes       entities.StringSlice `json:"scopes" validate:"required"`
	TokenExpiry  int                  `json:"token_expiry" validate:"omitempty,min=300,max=86400"`
	Confidential bool                 `json:"confidential"`
	SkipConsent  bool                 `json:"skip_consent"`
	FirstParty   bool                 `json:"first_party"`
	Active       bool                 `json:"active"`
}

type OAuthClientDeleteReq struct {
	ID int64 `param:"id" validate:"required"`
}
