package permissions

import (
	"anahita-libs/data/entities"
	"errors"
)

type hashtagsPermissions struct{}

func newHashtagsPermissions() PermissionsInterface {
	return &hashtagsPermissions{}
}

// CanBrowse returns true if the viewer is allowed to browse hashtags
func (p *hashtagsPermissions) CanBrowse(viewer *entities.Viewer) (bool, error) {
	return true, nil
}

// CanRead returns true if the viewer is allowed to read the hashtag
func (p *hashtagsPermissions) CanRead(viewer *entities.Viewer, entity interface{}) (bool, error) {
	return true, nil
}

// CanEdit returns true if the viewer is allowed to edit the hashtag
func (p *hashtagsPermissions) CanEdit(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New("you are not allowed to edit this hashtag")
}

// CanAdd returns true if the viewer is allowed to add the hashtag
func (p *hashtagsPermissions) CanAdd(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New("you are not allowed to add a hashtag")
}

// CanDelete returns true if the viewer is allowed to delete the hashtag
func (p *hashtagsPermissions) CanDelete(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New("you are not allowed to delete this hashtag")
}

// CanAdministrate returns true if the viewer is allowed to administrate the hashtag
func (p *hashtagsPermissions) CanAdministrate(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return true, errors.New("you are not allowed to administrate this hashtag")
}
