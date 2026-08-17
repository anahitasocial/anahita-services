package permissions

import (
	"anahita-libs/cmd/api/permissions"
)

func NewPermissions() *Permissions {
	return &Permissions{
		Medium: permissions.NewMediumPermissions(),
	}
}

type Permissions struct {
	Medium permissions.MediumPermissionsInterface
}
