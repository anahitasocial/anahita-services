package handlers

import (
	"anahita-libs/event"
	"comment/cmd/api/permissions"
	"comment/config"
	"comment/data/services"
)

type Env struct {
	Config    *config.Config
	Srv       *services.Services
	Perms     *permissions.Permissions
	Publisher *event.Publisher
}

type Handlers struct {
	Comment *commentHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Comment: NewCommentHandler(env),
	}
}
