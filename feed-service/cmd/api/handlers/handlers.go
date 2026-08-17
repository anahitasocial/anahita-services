package handlers

import (
	"anahita-libs/event"
	"feed/cmd/api/permissions"
	"feed/config"
	"feed/data/services"
)

type Env struct {
	Config    *config.Config
	Perms     *permissions.Permissions
	Srv       *services.Services
	Publisher *event.Publisher
}

type Handlers struct {
	FeedActor   *feedActorHandler
	FeedLeaders *feedLeadersHandler
	Repost      *repostHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		FeedActor:   NewFeedActorHandler(env),
		FeedLeaders: NewFeedLeadersHandler(env),
		Repost:      NewRepostHandler(env),
	}
}
