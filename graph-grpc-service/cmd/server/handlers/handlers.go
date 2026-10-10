package handlers

import (
	"database/sql"

	"anahita-libs/data/services"
	"anahita-libs/event"
	"graph/config"
	"graph/data/repositories"
)

type Env struct {
	Config *config.Config

	// DB is for the sweeper's lock, which has to be taken and released on one
	// connection. Everything else goes through Repos.
	DB *sql.DB

	Publisher *event.Publisher
	Repos     *repositories.Repositories

	// Files deletes uploaded avatars, covers and photos when an actor is
	// purged. Optional, exactly as it is for the sweeper: without
	// FILE_SERVICE_ADDR the purge still clears the database and leaves the
	// objects, which is better than refusing to purge at all.
	Files services.FilesGRPCInterface
}

type Handlers struct {
	Actor             *actorHandler
	ActorAdmin        *actorAdminHandler
	FeedActor         *feedActorHandler
	FeedLeaders       *feedLeadersHandler
	Graph             *graphHandler
	Like              *likeGrpcHandler
	Medium            *mediumHandler
	Node              *nodeHandler
	Edge              *edgeHandler
	Ping              *pingHandler
	Repost            *repostHandler
	Search            *searchHandler
	Socialgraph       *socialgraphGrpcHandler
	SocialgraphFollow *socialgraphFollowGrpcHandler
	Event             *eventGrpcHandler
	SocialgraphBlock  *socialgraphBlockGrpcHandler
	Subscription      *subscriptionGrpcHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Actor:             NewActorHandler(env),
		ActorAdmin:        NewActorAdminHandler(env),
		FeedActor:         NewFeedActorHandler(env),
		FeedLeaders:       NewFeedLeadersHandler(env),
		Graph:             NewGraphHandler(env),
		Like:              NewLikeGrpcHandler(env),
		Medium:            NewMediumHandler(env),
		Node:              NewNodeHandler(env),
		Edge:              NewEdgeHandler(env),
		Ping:              NewPingHandler(env),
		Repost:            NewRepostHandler(env),
		Search:            NewSearchHandler(env),
		Socialgraph:       NewSocialgraphGrpcHandler(env),
		SocialgraphFollow: NewSocialgraphFollowGrpcHandler(env),
		Event:             NewEventGrpcHandler(env),
		SocialgraphBlock:  NewSocialgraphBlockGrpcHandler(env),
		Subscription:      NewSubscriptionGrpcHandler(env),
	}
}
