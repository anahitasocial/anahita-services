package services

import (
	"anahita-libs/data/services"
	"feed/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:      services.NewActorsGRPC(config),
		LikesGRPC:       services.NewLikesGRPC(config),
		FeedLeadersGRPC: services.NewFeedLeadersGRPC(config),
		FeedActorGRPC:   services.NewFeedActorGRPC(config),
		FilesGRPC:       services.NewFilesGRPC(config),
		GraphNodeGRPC:   services.NewGraphNodeGRPC(config),
		RepostGRPC:      services.NewRepostGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	// Reads the owners of the posts in a feed, with the actor_features
	// defaults merged in, to answer who may comment and like.
	ActorsGRPC      services.ActorsGRPCInterface
	LikesGRPC       services.LikesGRPCInterface
	FeedLeadersGRPC services.FeedLeadersGRPCInterface
	FeedActorGRPC   services.FeedActorGRPCInterface
	FilesGRPC       services.FilesGRPCInterface
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	RepostGRPC      services.RepostGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
