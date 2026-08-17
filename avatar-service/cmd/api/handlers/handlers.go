package handlers

import (
	"anahita-libs/event"
	"avatar/cmd/api/permissions"
	"avatar/config"
	"avatar/data/services"
)

type Env struct {
	Config    *config.Config
	Srv       *services.Services
	Perms     *permissions.Permissions
	Publisher *event.Publisher
}

type Handlers struct {
	Avatar *avatarHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Avatar: NewAvatarHandler(env),
	}
}
