package permissions

import (
	"auth/data/entities"
	"errors"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"
)

const (
	// Scope errors — returned when the token lacks the required scope.
	// Map to HTTP 403 with error="insufficient_scope" per RFC 6750.
	ErrOAuthClientInsufficientScope = "insufficient scope"

	// Role errors — returned when the token has the right scope but
	// the authenticated user lacks the role the operation requires.
	ErrOAuthClientBrowseNotAllowed = "you are not allowed to browse OAuth clients"
	ErrOAuthClientReadNotAllowed   = "you are not allowed to read this OAuth client"
	ErrOAuthClientEditNotAllowed   = "you are not allowed to edit this OAuth client"
	ErrOAuthClientAddNotAllowed    = "you are not allowed to add an OAuth client"
	ErrOAuthClientDeleteNotAllowed = "you are not allowed to delete this OAuth client"

	// TOTP gate — write operations on the OAuth client registry
	// require two-factor authentication to be enabled on the viewer's
	// account. Reads and browse intentionally don't require this,
	// since read responses do not expose client secrets (verify this
	// remains true if you add fields to the response).
	ErrOAuthClientTotpRequired = "two-factor authentication must be enabled to manage OAuth clients"
)

type OAuthPerm struct {
	*Env
}

func NewOAuthPerm(env *Env) *OAuthPerm {
	return &OAuthPerm{
		env,
	}
}

// OAuth client management is restricted to global super admins:
//
//   - Scope gate (oauth_clients:read or :write): SuperAdminOnly at
//     token-mint time, so a non-super-admin will never have a token
//     with these scopes. Re-checked here as defense in depth.
//
//   - Role gate (IsSuperAdmin): redundant with the scope gate but
//     kept explicit so the intent is visible in the handler.
//
//   - TOTP gate (write only): destructive operations on the OAuth
//     client registry additionally require TOTP to be enabled on the
//     viewer's account. Read and browse are exempt because the read
//     response does not expose client secrets — if that changes, the
//     gate must be extended to reads as well.
//
// Read operations require oauth_clients:read; write operations require
// oauth_clients:write. The client argument on per-client methods is
// unused today (policy is uniform across all clients) but kept on the
// signature for future per-client checks (e.g., locking certain clients
// from edit).

func (prm *OAuthPerm) CanBrowseClients(viewer *libs_entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthClientsRead) {
		return false, errors.New(ErrOAuthClientInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthClientBrowseNotAllowed)
	}
	return true, nil
}

func (prm *OAuthPerm) CanReadClient(viewer *libs_entities.Viewer, client *entities.OAuthClient) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthClientsRead) {
		return false, errors.New(ErrOAuthClientInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthClientReadNotAllowed)
	}
	return true, nil
}

func (prm *OAuthPerm) CanEditClient(viewer *libs_entities.Viewer, client *entities.OAuthClient) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthClientsWrite) {
		return false, errors.New(ErrOAuthClientInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthClientEditNotAllowed)
	}
	if !viewer.TotpEnabled {
		return false, errors.New(ErrOAuthClientTotpRequired)
	}
	return true, nil
}

func (prm *OAuthPerm) CanAddClient(viewer *libs_entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthClientsWrite) {
		return false, errors.New(ErrOAuthClientInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthClientAddNotAllowed)
	}
	if !viewer.TotpEnabled {
		return false, errors.New(ErrOAuthClientTotpRequired)
	}
	return true, nil
}

func (prm *OAuthPerm) CanDeleteClient(viewer *libs_entities.Viewer, client *entities.OAuthClient) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthClientsWrite) {
		return false, errors.New(ErrOAuthClientInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthClientDeleteNotAllowed)
	}
	if !viewer.TotpEnabled {
		return false, errors.New(ErrOAuthClientTotpRequired)
	}
	return true, nil
}
