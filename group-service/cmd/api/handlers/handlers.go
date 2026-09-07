package handlers

import (
	"anahita-libs/event"
	"anahita-libs/sessions"
	"group/cmd/api/permissions"
	"group/config"
	"group/data/services"
)

type Env struct {
	Config    *config.Config
	Srv       *services.Services
	Perms     *permissions.Permissions
	Publisher *event.Publisher

	// Session reads the step-up marker auth-service writes at /reauth/*.
	//
	// group-service issues no proofs and never writes here. Deleting a group
	// destroys a shared space that other people's content lives in, which
	// makes it at least as worth proving as deleting your own profile.
	//
	// Optional: nil means the guard cannot verify, and requireStepUp refuses
	// rather than waving through. See the note there.
	Session sessions.KeyValueInterface
}

type Handlers struct {
	Admin *adminHandler
	Group *groupHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Admin: NewAdminHandler(env),
		Group: NewGroupHandler(env),
	}
}
