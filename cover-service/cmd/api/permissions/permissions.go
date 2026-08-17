package permissions

import (
	"anahita-libs/cmd/api/permissions"
)

func NewPermissions() *Permissions {
	return &Permissions{
		Actors: permissions.NewActorPermissions(),
		Media:  permissions.NewMediumPermissions(),
	}
}

type Permissions struct {
	Actors permissions.ActorPermissionsInterface
	Media  permissions.MediumPermissionsInterface
}
