package handlers

import (
	"search/cmd/api/permissions"
	"search/config"
	"search/data/services"
)

type Env struct {
	Config *config.Config
	Perms  *permissions.Permissions
	Srv    *services.Services
}

type Handlers struct {
	Search      *searchHandler
	Preferences *preferencesHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Search:      NewSearchHandler(env),
		Preferences: NewPreferencesHandler(env),
	}
}
