package handlers

import (
	"hashtag/cmd/api/permissions"
	"hashtag/config"
	"hashtag/data/services"
)

type Env struct {
	Config *config.Config
	Perms  *permissions.Permissions
	Srv    *services.Services
}

type Handlers struct {
	Hashtag *hashtagHandler
	Inbound *inboundHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Hashtag: NewHashtagHandler(env),
		Inbound: NewInboundHandler(env),
	}
}
