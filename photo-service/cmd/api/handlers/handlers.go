package handlers

import (
	"photo/cmd/api/permissions"
	"photo/config"
	"photo/data/services"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
	Perms  *permissions.Permissions
}

type Handlers struct {
	Photo *photoHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Photo: NewPhotoHandler(env),
	}
}
