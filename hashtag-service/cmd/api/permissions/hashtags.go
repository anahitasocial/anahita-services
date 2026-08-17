package permissions

import (
	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
)

const ErrHashtagInsufficientScope = "insufficient scope"

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
//
// The scope gate precedes the role check: taxonomy:write describes the
// API surface, the admin role describes the authority, and a role never
// bypasses a scope the token was not granted.
func (p *hashtagsPermissions) CanEdit(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeTaxonomyWrite) {
		return false, errors.New(ErrHashtagInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New("you are not allowed to edit this hashtag")
}

// CanAdd returns true if the viewer is allowed to add the hashtag
func (p *hashtagsPermissions) CanAdd(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeTaxonomyWrite) {
		return false, errors.New(ErrHashtagInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New("you are not allowed to add a hashtag")
}

// CanDelete returns true if the viewer is allowed to delete the hashtag
func (p *hashtagsPermissions) CanDelete(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeTaxonomyWrite) {
		return false, errors.New(ErrHashtagInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New("you are not allowed to delete this hashtag")
}

// CanAdministrate returns true if the viewer is allowed to administrate the hashtag
func (p *hashtagsPermissions) CanAdministrate(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeTaxonomyWrite) {
		return false, errors.New(ErrHashtagInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New("you are not allowed to administrate this hashtag")
}
