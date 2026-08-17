package permissions

import (
	"anahita-libs/cmd/api/permissions"
	"like/config"
	"like/data/services"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
}

func NewPermissions(env *Env) *Permissions {
	return &Permissions{
		Comments: permissions.NewCommentPermissions(),
		Likes:    NewLikePermissions(env),
		Medium:   permissions.NewMediumPermissions(),
	}
}

type Permissions struct {
	Comments permissions.CommentPermissionsInterface
	Likes    LikePermissionsInterface
	Medium   permissions.MediumPermissionsInterface
}
