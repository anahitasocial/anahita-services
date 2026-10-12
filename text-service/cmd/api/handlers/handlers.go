package handlers

import (
	"text/cmd/api/permissions"
	"text/config"
	"text/data/services"

	"anahita-libs/linkpreviews"
)

type Env struct {
	Config *config.Config
	Srv    *services.Services
	Perms  *permissions.Permissions
	// Previews of links, as link-preview-service read them. A note that
	// carries one takes its copy from here. Nil where the installation has
	// no cache to read: notes are then saved without.
	Previews *linkpreviews.Store
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
