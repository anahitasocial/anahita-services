package permissions

import (
	"anahita-libs/cmd/api/permissions"
)

func NewPermissions() *Permissions {
	return &Permissions{
		People: permissions.NewActorPermissions(),
		Groups: permissions.NewActorPermissions(),
		Actors: permissions.NewActorPermissions(),
	}
}

// Permissions answers what a viewer may do to an actor. One set of rules for
// every kind; the names say which kind the caller is asking about.
type Permissions struct {
	People permissions.ActorPermissionsInterface
	Groups permissions.ActorPermissionsInterface
	Actors permissions.ActorPermissionsInterface
}
