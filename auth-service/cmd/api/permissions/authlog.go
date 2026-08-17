package permissions

import (
	"errors"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"
	"auth/data/entities"
)

const (
	ErrAuthLogInsufficientScope      = "insufficient scope"
	ErrAuthLogNotAdminOrSuperAdmin   = "viewer is not admin or super admin"
	ErrAuthLogNotAllowedToAddAuthLog = "now allowed to add authentication log"
)

type AuthLogPerm struct {
	*Env
}

// NewAuthLogPerm creates a new AuthLogPerm instance
func NewAuthLogPerm(env *Env) *AuthLogPerm {
	return &AuthLogPerm{
		env,
	}
}

// CanBrowse checks if the viewer can browse the authentication logs of a person
func (prm *AuthLogPerm) CanBrowse(viewer *libs_entities.Viewer, user *entities.User) error {
	if !viewer.HasScope(libs_constants.OAuthScopeAccountRead) {
		return errors.New(ErrAuthLogInsufficientScope)
	}

	if viewer.ID != user.ID {
		if !viewer.IsAdminOrSuperAdmin() {
			err := errors.New(ErrAuthLogNotAdminOrSuperAdmin)
			return err
		}
	}

	return nil
}

// CanAdd checks if the viewer can add an authentication log
func (prm *AuthLogPerm) CanAdd(user *entities.User) error {
	if user.ID == 0 {
		err := errors.New(ErrAuthLogNotAllowedToAddAuthLog)
		return err
	}

	return nil
}

// CanDelete checks if the viewer can delete an authentication log of a person
func (prm *AuthLogPerm) CanDelete(viewer *libs_entities.Viewer, user *entities.User) error {
	if !viewer.HasScope(libs_constants.OAuthScopeAccountWrite) {
		return errors.New(ErrAuthLogInsufficientScope)
	}

	if viewer.ID != user.ID {
		if !viewer.IsAdminOrSuperAdmin() {
			err := errors.New(ErrAuthLogNotAdminOrSuperAdmin)
			return err
		}
	}

	return nil
}
