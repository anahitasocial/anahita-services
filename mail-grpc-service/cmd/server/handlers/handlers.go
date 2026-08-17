package handlers

import (
	"mail/config"
	"mail/providers"
)

type Env struct {
	Config   *config.Config
	Provider providers.MailProviderInterface
}

type Handlers struct {
	Mail *mailHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Mail: NewMailHandler(env),
	}
}
