package handlers

import (
	"anahita-libs/event"
	"group/cmd/api/permissions"
	"group/config"
	"group/data/services"
)

type Env struct {
	Config    *config.Config
	Srv       *services.Services
	Perms     *permissions.Permissions
	Publisher *event.Publisher
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
