package handlers

import (
	"video/cmd/api/permissions"
	"video/config"
	"video/data/services"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
	Perms  *permissions.Permissions
}

type Handlers struct {
	Video *videoHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Video: NewVideoHandler(env),
	}
}
