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

	// TOTP gate — write operations on the signing-key registry
	// require two-factor authentication to be enabled on the viewer's
	// account. Reads and browse are exempt because the read response
	// exposes only metadata (kid, active flag, public key for JWKS);
	// the private signing material lives in AWS KMS and is never
	// returned. If that changes, the gate must be extended to reads.
	ErrOAuthSigningKeyTotpRequired = "two-factor authentication must be enabled to manage OAuth signing keys"
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
//   - TOTP gate (Edit only): toggling a signing key's active flag is
//     a platform-critical operation — deactivating the live key
//     breaks token signing, and reactivating a rotated-out key could
//     re-enable forgery if that key was previously leaked. Requires
//     TOTP on the viewer's account. Read and browse are exempt
//     because the read response exposes only metadata.
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
	if !viewer.TotpEnabled {
		return false, errors.New(ErrOAuthSigningKeyTotpRequired)
	}
	return true, nil
}
