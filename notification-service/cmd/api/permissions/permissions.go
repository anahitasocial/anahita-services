package permissions

import (
	an_perms "anahita-libs/permissions"
)

func NewPermissions() *Permissions {
	return &Permissions{
		Actors:        an_perms.NewActorPermissions(),
		Media:         an_perms.NewMediumPermissions(),
		Notifications: newNotificationsPermissions(),
	}
}

type Permissions struct {
	Actors        an_perms.ActorPermissionsInterface
	Media         an_perms.MediumPermissionsInterface
	Notifications *notificationsPermissions
}
