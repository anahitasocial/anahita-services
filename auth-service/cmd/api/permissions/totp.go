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
func (prm *TotpPerm) CanDelete(viewer *libs_entities.Viewer, user *entities.User) error {
	// Self-service disable.
	if viewer.ID == user.ID {
		return nil
	}

	// Cross-user disable requires the actors:write scope.
	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrTotpInsufficientScope)
	}

	// Global admins (admin or super_admin from HQ org) can disable for anyone.
	if viewer.IsSuperAdmin() {
		return nil
	}

	return errors.New(ErrTotpNoUserPermissions)
}
