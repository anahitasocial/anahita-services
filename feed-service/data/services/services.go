package services

import (
	"anahita-libs/data/services"
	"feed/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
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
	LikesGRPC       services.LikesGRPCInterface
	FeedLeadersGRPC services.FeedLeadersGRPCInterface
	FeedActorGRPC   services.FeedActorGRPCInterface
	FilesGRPC       services.FilesGRPCInterface
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	RepostGRPC      services.RepostGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
