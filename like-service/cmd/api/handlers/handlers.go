package handlers

import (
	"anahita-libs/event"
	"like/cmd/api/permissions"
	"like/config"
	"like/data/services"
)

type Env struct {
	Config    *config.Config
	Srv       *services.Services
	Perms     *permissions.Permissions
	Publisher *event.Publisher
}

type Handlers struct {
	Like *likeHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Like: NewLikeHandler(env),
	}
}
