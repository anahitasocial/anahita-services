package handlers

import (
	"anahita-libs/ratelimit"
	"video/cmd/api/permissions"
	"video/config"
	"video/data/services"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
	Perms  *permissions.Permissions
	// Counts downloads, so that one person cannot ask for them without end.
	// nil where there is nothing to count with: downloads are then not
	// limited.
	Limiter ratelimit.Counter
}

type Handlers struct {
	Video *videoHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Video: NewVideoHandler(env),
	}
}
