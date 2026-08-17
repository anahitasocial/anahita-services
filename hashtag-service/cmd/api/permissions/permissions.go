package permissions

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"hashtag/config"
)

type Env struct {
	Config *config.Config
}

type PermissionsInterface interface {
	CanBrowse(viewer *entities.Viewer) (bool, error)
	CanRead(viewer *entities.Viewer, entity interface{}) (bool, error)
	CanEdit(viewer *entities.Viewer, entity interface{}) (bool, error)
	CanAdd(viewer *entities.Viewer, entity interface{}) (bool, error)
	CanDelete(viewer *entities.Viewer, entity interface{}) (bool, error)
	CanAdministrate(viewer *entities.Viewer, entity interface{}) (bool, error)
}

func NewPermissions() *Permissions {
	return &Permissions{
		Hashtags:  newHashtagsPermissions(),
		Taggables: permissions.NewTaggablePermissions(),
	}
}

type Permissions struct {
	Hashtags  PermissionsInterface
	Taggables permissions.TaggablePermissionsInterface
}
