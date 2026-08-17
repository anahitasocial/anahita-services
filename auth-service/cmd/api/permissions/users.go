package permissions

import (
	anEntities "anahita-libs/data/entities"
	"auth/constants"
	"auth/data/entities"
	"errors"
)

type UsersPermissions struct{}

func NewUsersPermissions(env *Env) *UsersPermissions {
	return &UsersPermissions{}
}

func (p *UsersPermissions) CanBrowse(viewer *anEntities.Viewer) (bool, error) {
	if !viewer.HasScope("admin:actors") {
		return false, errors.New(constants.ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(constants.ErrUserBrowseNotAllowed)
}

func (p *UsersPermissions) CanRead(viewer *anEntities.Viewer, entity interface{}) (bool, error) {
	if viewer.ID == entity.(*entities.User).ID {
		if !viewer.HasScope("read:account") {
			return false, errors.New(constants.ErrInsufficientScope)
		}
		return true, nil
	}

	if !viewer.HasScope("admin:actors") {
		return false, errors.New(constants.ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(constants.ErrUserReadNotAllowed)
}

func (p *UsersPermissions) CanEdit(viewer *anEntities.Viewer, entity interface{}) (bool, error) {
	if viewer.ID == entity.(*entities.User).ID {
		if !viewer.HasScope("write:account") {
			return false, errors.New(constants.ErrInsufficientScope)
		}
		return true, nil
	}

	if !viewer.HasScope("admin:actors") {
		return false, errors.New(constants.ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(constants.ErrUserEditNotAllowed)
}

func (p *UsersPermissions) CanAdd(viewer *anEntities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope("admin:actors") {
		return false, errors.New(constants.ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(constants.ErrUserAddNotAllowed)
}

func (p *UsersPermissions) CanDelete(viewer *anEntities.Viewer, entity interface{}) (bool, error) {
	user := entity.(*entities.User)

	if viewer.ID > 0 && viewer.ID == user.ID {
		if !viewer.HasScope("write:account") {
			return false, errors.New(constants.ErrInsufficientScope)
		}
		return true, nil
	}

	if !viewer.HasScope("admin:actors") {
		return false, errors.New(constants.ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(constants.ErrUserDeleteNotAllowed)
}

func (p *UsersPermissions) CanAdministrate(viewer *anEntities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope("admin:actors") {
		return false, errors.New(constants.ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(constants.ErrUserAdministrateNotAllowed)
}
