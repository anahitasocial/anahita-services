package handlers

import (
	"anahita-libs/event"
	"cover/cmd/api/permissions"
	"cover/config"
	"cover/data/services"
)

type Env struct {
	Config    *config.Config
	Srv       *services.Services
	Perms     *permissions.Permissions
	Publisher *event.Publisher
}

type Handlers struct {
	Cover *coverHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Cover: NewCoverHandler(env),
	}
}
