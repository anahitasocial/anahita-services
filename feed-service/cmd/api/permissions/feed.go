package permissions

import (
	"anahita-libs/data/entities"
	"errors"
)

type feedPermissions struct {
	*Env
}

func NewFeedPermissions(env *Env) *feedPermissions {
	return &feedPermissions{
		Env: env,
	}
}

// CanBrowse - feed is public content
func (p *feedPermissions) CanBrowse(viewer *entities.Viewer) (bool, error) {
	return true, nil
}

// CanEdit - feeds are read-only computed views
func (p *feedPermissions) CanEdit(viewer *entities.Viewer, entity interface{}) (bool, error) {
	return false, errors.New(ErrFeedEditNotAllowed)
}

// CanDelete - feeds are read-only computed views
func (p *feedPermissions) CanDelete(viewer *entities.Viewer, entity interface{}) (bool, error) {
	return false, errors.New(ErrFeedDeleteNotAllowed)
}

// CanAdministrate - only site admins
func (p *feedPermissions) CanAdministrate(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope("admin:media") {
		return false, errors.New(ErrInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(ErrFeedAdminNotAllowed)
}
