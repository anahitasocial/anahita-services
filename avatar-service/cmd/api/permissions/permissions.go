package permissions

import (
	"anahita-libs/cmd/api/permissions"
)

func NewPermissions() *Permissions {
	return &Permissions{
		Actors: permissions.NewActorPermissions(),
	}
}

type Permissions struct {
	Actors permissions.ActorPermissionsInterface
}
