package permissions

import (
	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
)

const (
	ErrInsufficientScope              = "insufficient scope"
	ErrLocationEditNotAllowed         = "you are not allowed to edit this location"
	ErrLocationAddNotAllowed          = "you are not allowed to add a location"
	ErrLocationDeleteNotAllowed       = "you are not allowed to delete this location"
	ErrLocationAdministrateNotAllowed = "you are not allowed to administrate this location"
)

type locationsPermissions struct{}

func newLocationsPermissions() PermissionsInterface {
	return &locationsPermissions{}
}

func (p *locationsPermissions) CanBrowse(viewer *entities.Viewer) (bool, error) {
	return true, nil
}

func (p *locationsPermissions) CanRead(viewer *entities.Viewer, entity interface{}) (bool, error) {
	return true, nil
}

func (p *locationsPermissions) CanEdit(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeTaxonomyWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(ErrLocationEditNotAllowed)
}

func (p *locationsPermissions) CanAdd(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeTaxonomyWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(ErrLocationAddNotAllowed)
}

func (p *locationsPermissions) CanDelete(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeTaxonomyWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(ErrLocationDeleteNotAllowed)
}

func (p *locationsPermissions) CanAdministrate(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeTaxonomyWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(ErrLocationAdministrateNotAllowed)
}
