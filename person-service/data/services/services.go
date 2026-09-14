package services

import (
	"anahita-libs/data/services"
	"person/config"
)

func NewServices(config *config.Config) *Services {
	nodes := services.NewGraphNodeGRPC(config)
	edges := services.NewGraphEdgeGRPC(config)

	return &Services{
		ActorsGRPC:      services.NewActorsGRPC(config),
		FilesGRPC:       services.NewFilesGRPC(config),
		GraphEdgeGRPC:   edges,
		GraphNodeGRPC:   nodes,
		Mentions:        NewMentions(nodes, edges),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	ActorsGRPC    services.ActorsGRPCInterface
	FilesGRPC     services.FilesGRPCInterface
	GraphEdgeGRPC services.GraphEdgeGRPCInterface
	GraphNodeGRPC services.GraphNodeGRPCInterface
	// Mentions keeps a node's mention edges in step with the @handles in its
	// body. Built on the two clients above rather than on a repository,
	// because `edges` belongs to graph-grpc-service.
	Mentions        MentionsInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
