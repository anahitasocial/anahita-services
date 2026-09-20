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
//   - Scope gate (oauth:read or :write): SuperAdminOnly at
//     token-mint time, so a non-super-admin will never have a token
//     with these scopes. Re-checked here as defense in depth.
//
//   - Role gate (IsSuperAdmin): redundant with the scope gate but
//     kept explicit so the intent is visible in the handler.
//
// THESE ANSWER ONLY "is this the right person". Whether that person is
// actually at the keyboard is a separate question, asked by the
// handlers through requireStepUp.
//
// There used to be a third gate here, `if !viewer.TotpEnabled`, on each
// of the three writes. It denied everyone. The viewer is built from JWT
// claims and there is no TOTP claim, so the field read false on every
// authenticated request no matter what the account had enrolled —
// exactly the trap hydrateViewer's comment describes. All three write
// endpoints were unreachable.
//
// It is not repaired by hydrating, because enrolment was the wrong
// question anyway: "has TOTP configured" is a claim about the account,
// while a session left open on an unattended laptop satisfies it just
// as well as the owner does. The handlers now require a step-up
// instead — a proof, within five minutes, by passkey, passcode or
// password. That also answers the reconciliation note in
// permissions/web_authn.go, and it is what lets a passkey stand in.
//
// Read operations require oauth:read; write operations require
// oauth:write. The client argument on per-client methods is
// unused today (policy is uniform across all clients) but kept on the
// signature for future per-client checks (e.g., locking certain clients
// from edit).

func (prm *OAuthPerm) CanBrowseClients(viewer *libs_entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthRead) {
		return false, errors.New(ErrOAuthClientInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthClientBrowseNotAllowed)
	}
	return true, nil
}

func (prm *OAuthPerm) CanReadClient(viewer *libs_entities.Viewer, client *entities.OAuthClient) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthRead) {
		return false, errors.New(ErrOAuthClientInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthClientReadNotAllowed)
	}
	return true, nil
}

func (prm *OAuthPerm) CanEditClient(viewer *libs_entities.Viewer, client *entities.OAuthClient) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthWrite) {
		return false, errors.New(ErrOAuthClientInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthClientEditNotAllowed)
	}
	return true, nil
}

func (prm *OAuthPerm) CanAddClient(viewer *libs_entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthWrite) {
		return false, errors.New(ErrOAuthClientInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthClientAddNotAllowed)
	}
	return true, nil
}

func (prm *OAuthPerm) CanDeleteClient(viewer *libs_entities.Viewer, client *entities.OAuthClient) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthWrite) {
		return false, errors.New(ErrOAuthClientInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthClientDeleteNotAllowed)
	}
	return true, nil
}
