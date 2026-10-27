package permissions

import (
	"anahita-libs/permissions"
	"like/config"
	"like/data/services"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
}

func NewPermissions(env *Env) *Permissions {
	return &Permissions{
		Likes:  NewLikePermissions(env),
		Medium: permissions.NewMediumPermissions(),
	}
}

type Permissions struct {
	Likes  LikePermissionsInterface
	Medium permissions.MediumPermissionsInterface
}
