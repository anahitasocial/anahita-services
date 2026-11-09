package handlers

import (
	"anahita-libs/ratelimit"
	"video/cmd/api/permissions"
	"video/config"
	"video/data/repositories"
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
	// Videos from other networks copied here when played (elsewhere.go),
	// and whether a server is one this site refuses.
	Elsewhere repositories.ElsewhereInterface
	Refused   func(host string) bool
}

type Handlers struct {
	Video *videoHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Video: NewVideoHandler(env),
	}
}
