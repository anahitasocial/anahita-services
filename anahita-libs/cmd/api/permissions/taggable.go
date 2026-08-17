package permissions

import (
	"anahita-libs/data/entities"
	"errors"
)

const (
	ErrTaggableInsufficientScope = "insufficient scope"
	ErrTaggableEditNotAllowed    = "you are not allowed to edit this tag"
	ErrTaggableAddNotAllowed     = "you are not allowed to add a tag"
	ErrTaggableDeleteNotAllowed  = "you are not allowed to delete this tag"
	ErrTaggableAdminNotAllowed   = "you are not allowed to administrate this tag"
)

type TaggableInterface interface {
	GetID() int64
	GetAuthorID() int64
	GetOwnerID() int64
	GetParentID() int64
}

type TaggablePermissionsInterface interface {
	CanBrowse(viewer *entities.Viewer) (bool, error)
	CanEdit(viewer *entities.Viewer, taggable TaggableInterface) (bool, error)
	CanAdd(viewer *entities.Viewer, taggable TaggableInterface) (bool, error)
	CanDelete(viewer *entities.Viewer, taggable TaggableInterface) (bool, error)
	CanAdministrate(viewer *entities.Viewer, taggable TaggableInterface) (bool, error)
}

type TaggablePermissions struct{}

func NewTaggablePermissions() TaggablePermissionsInterface {
	return &TaggablePermissions{}
}

// CanBrowse - hashtags on content are publicly visible
func (t *TaggablePermissions) CanBrowse(viewer *entities.Viewer) (bool, error) {
	return true, nil
}

// CanEdit - tagging is add/delete, not edit
func (t *TaggablePermissions) CanEdit(viewer *entities.Viewer, taggable TaggableInterface) (bool, error) {
	return false, errors.New(ErrTaggableEditNotAllowed)
}

// CanAdd - author, owner, or admin can tag content
func (t *TaggablePermissions) CanAdd(viewer *entities.Viewer, taggable TaggableInterface) (bool, error) {
	if !viewer.HasScope("write:media") {
		return false, errors.New(ErrTaggableInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	if viewer.ID == taggable.GetAuthorID() {
		return true, nil
	}

	if viewer.ID == taggable.GetOwnerID() {
		return true, nil
	}

	return false, errors.New(ErrTaggableAddNotAllowed)
}

// CanDelete - author, owner, or admin can remove tags
func (t *TaggablePermissions) CanDelete(viewer *entities.Viewer, taggable TaggableInterface) (bool, error) {
	if !viewer.HasScope("write:media") {
		return false, errors.New(ErrTaggableInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	if viewer.ID == taggable.GetAuthorID() {
		return true, nil
	}

	if viewer.ID == taggable.GetOwnerID() {
		return true, nil
	}

	return false, errors.New(ErrTaggableDeleteNotAllowed)
}

// CanAdministrate - admin only
func (t *TaggablePermissions) CanAdministrate(viewer *entities.Viewer, taggable TaggableInterface) (bool, error) {
	if !viewer.HasScope("admin:media") {
		return false, errors.New(ErrTaggableInsufficientScope)
	}

	if viewer.IsAdmin() || viewer.IsSuperAdmin() {
		return true, nil
	}

	return false, errors.New(ErrTaggableAdminNotAllowed)
}
