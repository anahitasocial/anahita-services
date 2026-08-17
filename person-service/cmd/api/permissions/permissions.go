package permissions

import (
	"anahita-libs/cmd/api/permissions"
)

func NewPermissions() *Permissions {
	return &Permissions{
		People: permissions.NewActorPermissions(),
	}
}

type Permissions struct {
	People permissions.ActorPermissionsInterface
}
