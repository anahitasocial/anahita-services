package handlers

import (
	"text/cmd/api/permissions"
	"text/config"
	"text/data/services"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
	Perms  *permissions.Permissions
}

type Handlers struct {
	Article *articleHandler
	Note    *noteHandler
	Topic   *topicHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Article: NewArticleHandler(env),
		Note:    NewNoteHandler(env),
		Topic:   NewTopicHandler(env),
	}
}
