package permissions

import (
	"errors"

	"auth/data/entities"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"
)

const (
	// Scope errors — returned when the token lacks the required scope.
	// Map to HTTP 403 with error="insufficient_scope" per RFC 6750.
	ErrOAuthSigningKeyInsufficientScope = "insufficient scope"

	// Role errors — returned when the token has the right scope but
	// the authenticated user lacks the role the operation requires.
	ErrOAuthSigningKeyBrowseNotAllowed = "you are not allowed to browse OAuth signing keys"
	ErrOAuthSigningKeyReadNotAllowed   = "you are not allowed to read this OAuth signing key"
	ErrOAuthSigningKeyEditNotAllowed   = "you are not allowed to edit this OAuth signing key"
)

type OAuthSigningKeyPerm struct {
	*Env
}

func NewOAuthSigningKeyPerm(env *Env) *OAuthSigningKeyPerm {
	return &OAuthSigningKeyPerm{
		env,
	}
}

// OAuth signing-key management is restricted to global super admins:
//
//   - Scope gate (oauth:read or :write): SuperAdminOnly
//     at token-mint time, so a non-super-admin will never have a
//     token with these scopes. Re-checked here as defense in depth.
//
//   - Role gate (IsSuperAdmin): redundant with the scope gate
//     but kept explicit so the intent is visible in the handler.
//
// Toggling a key's active flag is platform-critical — deactivating the
// live key breaks token signing, and reactivating a rotated-out key
// could re-enable forgery if that key ever leaked — so Edit is
// additionally guarded. That guard is a STEP-UP, applied by the
// handler, not a gate here: these functions answer whether this is the
// right person, and requireStepUp answers whether they are present.
//
// It used to be `if !viewer.TotpEnabled` in this file, which denied
// everyone — the viewer comes from JWT claims and carries no TOTP
// claim, so it read false on every request. See the longer note in
// permissions/oauth.go; the same bug was in both, and both now take
// a proof by passkey, passcode or password instead of an enrolment
// flag that was never populated.
//
// Read and browse stay unguarded beyond the role: the response exposes
// only metadata (kid, active flag, algorithm), never key material.
//
// Read operations require oauth:read; Edit (the only
// mutation, toggling active) requires oauth:write. Add
// and Delete are not exposed here by design — provisioning happens
// through deploy + SeedKMSSigningKey, cleanup through DeleteExpired.
//
// The key argument on Read/Edit is unused today (the policy is
// uniform across all keys) but kept on the signature for future
// per-key checks (e.g., locking a kid from edit during an active
// rotation).

func (prm *OAuthSigningKeyPerm) CanBrowseSigningKeys(viewer *libs_entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthRead) {
		return false, errors.New(ErrOAuthSigningKeyInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthSigningKeyBrowseNotAllowed)
	}
	return true, nil
}

func (prm *OAuthSigningKeyPerm) CanReadSigningKey(viewer *libs_entities.Viewer, key *entities.OAuthSigningKey) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthRead) {
		return false, errors.New(ErrOAuthSigningKeyInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthSigningKeyReadNotAllowed)
	}
	return true, nil
}

func (prm *OAuthSigningKeyPerm) CanEditSigningKey(viewer *libs_entities.Viewer, key *entities.OAuthSigningKey) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeOAuthWrite) {
		return false, errors.New(ErrOAuthSigningKeyInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return false, errors.New(ErrOAuthSigningKeyEditNotAllowed)
	}
	return true, nil
}
