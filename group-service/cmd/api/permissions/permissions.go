package permissions

import (
	"anahita-libs/cmd/api/permissions"
)

func NewPermissions() *Permissions {
	return &Permissions{
		Groups: permissions.NewActorPermissions(),
		Actors: permissions.NewActorPermissions(),
	}
}

type Permissions struct {
	Groups permissions.ActorPermissionsInterface
	Actors permissions.ActorPermissionsInterface
}
