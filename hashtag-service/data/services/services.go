package services

import (
	"anahita-libs/data/services"
	"hashtag/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		FilesGRPC:     services.NewFilesGRPC(config),
		GraphGRPC:     services.NewGraphGRPC(config),
		GraphEdgeGRPC: services.NewGraphEdgeGRPC(config),
		GraphNodeGRPC: services.NewGraphNodeGRPC(config),
		HashtagGRPC:   services.NewHashtagGRPC(config),
	}
}

type Services struct {
	FilesGRPC     services.FilesGRPCInterface
	GraphGRPC     services.GraphGRPCInterface
	GraphEdgeGRPC services.GraphEdgeGRPCInterface
	GraphNodeGRPC services.GraphNodeGRPCInterface
	HashtagGRPC   services.HashtagGRPCInterface
}
