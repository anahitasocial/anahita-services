package permissions

import (
	"anahita-libs/data/entities"
)

type PermissionsInterface interface {
	CanBrowse(viewer *entities.Viewer) (bool, error)
}

func NewPermissions() *Permissions {
	return &Permissions{
		Search: newSearchPermissions(),
	}
}

type Permissions struct {
	Search PermissionsInterface
}
