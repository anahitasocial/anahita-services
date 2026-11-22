package permissions

import (
	"anahita-libs/data/entities"
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
