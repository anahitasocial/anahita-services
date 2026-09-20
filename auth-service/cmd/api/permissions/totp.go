package permissions

import (
	"errors"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"
	"auth/data/entities"
)

const (
	ErrTotpNoUserPermissions  = "you do not have the right user permissions"
	ErrTotpTotpNotInitiated   = "totp isn't initiated"
	ErrTotpTotpAlreadyEnabled = "there is an TOTP already enabled. You must disable it first before enabling a new TOTP"
	ErrTotpInsufficientScope  = "insufficient scope"
)

type TotpPerm struct {
	*Env
}

func NewTotpPerm(env *Env) *TotpPerm {
	return &TotpPerm{
		env,
	}
}

// Can viewer Enable TOTP?
func (prm *TotpPerm) CanEdit(viewer *libs_entities.Viewer, user *entities.User) error {
	if viewer.ID != user.ID {
		err := errors.New(ErrTotpNoUserPermissions)
		return err
	}

	return nil
}

// Can viewer create a TOTP secret?
func (prm *TotpPerm) CanAdd(viewer *libs_entities.Viewer, user *entities.User) error {
	if viewer.ID != user.ID {
		err := errors.New(ErrTotpNoUserPermissions)
		return err
	}

	return nil
}

// Can viewer add recovery codes
func (prm *TotpPerm) CanAddRecoveryCodes(viewer *libs_entities.Viewer, user *entities.User) error {
	if viewer.ID != user.ID {
		err := errors.New(ErrTotpNoUserPermissions)
		return err
	}

	return nil
}

// Can viewer disable Person's TOTP 2FA
// CanRead gates reading whether somebody has TOTP enabled.
//
// Your own, always — GET /totp answers that and takes no id. Somebody
// else's needs an administrator, matching CanDelete: an administration
// screen that offers to remove a second factor has to be able to say
// whether there is one, or it offers a button whose effect nobody can
// predict.
//
// Enabled and expired only. Never the secret, never the recovery codes:
// this says whether a factor exists, not what it is.
func (prm *TotpPerm) CanRead(viewer *libs_entities.Viewer, user *entities.User) error {
	if viewer.ID == user.ID {
		return nil
	}

	if !viewer.HasScope(libs_constants.OAuthScopeAccountRead) {
		return errors.New(ErrTotpInsufficientScope)
	}

	if viewer.IsAdminOrSuperAdmin() {
		return nil
	}

	return errors.New(ErrTotpNoUserPermissions)
}

func (prm *TotpPerm) CanDelete(viewer *libs_entities.Viewer, user *entities.User) error {
	// Self-service disable.
	if viewer.ID == user.ID {
		return nil
	}

	// Cross-user disable requires the actors:write scope.
	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrTotpInsufficientScope)
	}

	// Administrators and super-administrators alike. Locking a person out of
	// their own account is the failure this exists to undo — a lost phone with
	// no recovery codes left — and routing every one of those through a single
	// super-admin makes the recovery slower than the problem.
	if viewer.IsAdminOrSuperAdmin() {
		return nil
	}

	return errors.New(ErrTotpNoUserPermissions)
}
