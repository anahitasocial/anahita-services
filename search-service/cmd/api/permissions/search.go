package permissions

import (
	"anahita-libs/data/entities"
)

type searchPermissions struct{}

func newSearchPermissions() PermissionsInterface {
	return &searchPermissions{}
}

// CanBrowse returns true if the viewer is allowed to browse storys
func (p *searchPermissions) CanBrowse(viewer *entities.Viewer) (bool, error) {
	return true, nil
}
