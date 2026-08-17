package handlers

import (
	"anahita-libs/event"
	"person/cmd/api/permissions"
	"person/config"
	"person/data/repositories"
	"person/data/services"
)

type Env struct {
	Config    *config.Config
	Repos     *repositories.Repositories
	Srv       *services.Services
	Perms     *permissions.Permissions
	Publisher *event.Publisher
}

type Handlers struct {
	Person *personHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Person: NewPersonHandler(env),
	}
}
