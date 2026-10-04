package entities

import (
	"anahita-libs/constants"
	"slices"
	"time"
)

type Viewer struct {
	AnahitaNode

	// OAuth fields
	Authenticated bool
	Scopes        []string
	ClientID      string

	// OAuthRefreshTokenID is the BIGINT id of the oauth_refresh_tokens
	// row that minted this viewer's current access token. Populated
	// from the refresh_token_id JWT claim. Used to identify the
	// viewer's current session — e.g. the auth log UI flags the row
	// that matches as "this device".
	//
	// Zero for client_credentials grants (no refresh token), or for
	// older tokens minted before the claim existed.
	OAuthRefreshTokenID int64

	Enabled       bool      `json:"enabled"`
	PersonType    string    `json:"usertype"`
	Email         string    `json:"email"`
	Username      string    `json:"username"`
	LastVisitedAt time.Time `json:"last_visited_at,omitempty"`

	// TOTP fields
	TotpSecret         string
	TotpRecoveryCodes  string
	TotpEnabled        bool
	TotpLastVerifiedAt time.Time
}

func NewViewer() *Viewer {
	v := &Viewer{
		Authenticated: false,
		Scopes:        []string{},
		ClientID:      "",
	}
	v.Type = constants.NodeActorPersonEntityType
	v.Service = constants.PersonService
	v.Enabled = false
	v.PersonType = constants.PersonTypeGuest
	v.LastVisitedAt = time.Now()

	return v
}

func (v *Viewer) IsGuest() bool {
	return v.PersonType == constants.PersonTypeGuest
}

func (v *Viewer) IsRegistered() bool {
	return v.PersonType == constants.PersonTypeRegistered
}

func (v *Viewer) IsAdmin() bool {
	return v.IsSuperAdmin() || v.PersonType == constants.PersonTypeAdmin
}

func (v *Viewer) IsSuperAdmin() bool {
	return v.PersonType == constants.PersonTypeSuperAdmin
}

func (v *Viewer) IsAdminOrSuperAdmin() bool {
	return v.IsAdmin() || v.IsSuperAdmin()
}

func (v *Viewer) HasScope(scope string) bool {
	return slices.Contains(v.Scopes, scope)
}

func (v *Viewer) RequireScope(scope string) bool {
	if !v.Authenticated {
		return true // guests bypass scope checks
	}
	return v.HasScope(scope)
}
