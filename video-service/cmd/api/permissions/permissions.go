package permissions

import (
	"anahita-libs/permissions"
)

func NewPermissions() *Permissions {
	return &Permissions{
		Media: permissions.NewMediumPermissions(),
	}
}

type Permissions struct {
	Media permissions.MediumPermissionsInterface
}
