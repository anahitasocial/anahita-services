package handlers

import (
	"location/cmd/api/permissions"
	"location/config"
	"location/data/services"
)

type Env struct {
	Config *config.Config
	Perms  *permissions.Permissions
	Srv    *services.Services
}

type Handlers struct {
	Location *locationHandler
	Inbound  *inboundHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Location: NewLocationHandler(env),
		Inbound:  NewInboundHandler(env),
	}
}
