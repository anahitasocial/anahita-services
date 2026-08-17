package handlers

import (
	"anahita-libs/event"
	"graph/config"
	"graph/data/repositories"
)

type Env struct {
	Config    *config.Config
	Publisher *event.Publisher
	Repos     *repositories.Repositories
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
		SocialgraphBlock:  NewSocialgraphBlockGrpcHandler(env),
		Subscription:      NewSubscriptionGrpcHandler(env),
	}
}
