package handlers

import (
	"identityimage/cmd/api/permissions"
	"identityimage/config"
	"identityimage/data/services"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
	Perms  *permissions.Permissions
}

// Handlers is the one handler this service has. An avatar and a cover are
// the same work with a different name on it, so they are one piece of code
// told which kind it is doing.
type Handlers struct {
	Image *imageHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Image: NewImageHandler(env),
	}
}
