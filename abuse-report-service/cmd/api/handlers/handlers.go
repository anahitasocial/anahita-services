package handlers

import (
	"abusereport/cmd/api/permissions"
	"abusereport/config"
	"abusereport/data/reasons"
	"abusereport/data/repositories"
	"abusereport/data/services"

	"anahita-libs/ratelimit"
)

type Env struct {
	Config  *config.Config
	Perms   *permissions.Permissions
	Repos   *repositories.Repositories
	Srv     *services.Services
	Reasons *reasons.Source
	// Limiter counts reports per reporter. Redis, through the session store.
	Limiter ratelimit.Counter
}

type Handlers struct {
	Reasons *reasonsHandler
	Reports *reportsHandler
	Cases   *casesHandler
	Events  *eventsHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Reasons: &reasonsHandler{env},
		Reports: &reportsHandler{env},
		Cases:   &casesHandler{env},
		Events:  &eventsHandler{env},
	}
}
