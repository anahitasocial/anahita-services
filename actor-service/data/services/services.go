package services

import (
	"actor/config"
	"anahita-libs/data/services"
)

func NewServices(config *config.Config) *Services {
	nodes := services.NewGraphNodeGRPC(config)
	edges := services.NewGraphEdgeGRPC(config)

	return &Services{
		ActorsGRPC:            services.NewActorsGRPC(config),
		ActorAdminsGRPC:       services.NewActorAdminsGRPC(config),
		EventsGRPC:            services.NewEventsGRPC(config),
		FilesGRPC:             services.NewFilesGRPC(config),
		GraphEdgeGRPC:         edges,
		GraphNodeGRPC:         nodes,
		Mentions:              NewMentions(nodes, edges),
		SocialgraphFollowGRPC: services.NewSocialgraphFollowGRPC(config),
		SocialgraphGRPC:       services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	ActorsGRPC services.ActorsGRPCInterface
	// The administrators of a group.
	ActorAdminsGRPC services.ActorAdminsGRPCInterface
	EventsGRPC      services.EventsGRPCInterface
	FilesGRPC       services.FilesGRPCInterface
	GraphEdgeGRPC   services.GraphEdgeGRPCInterface
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	// Mentions keeps a node's mention edges in step with the @handles in its
	// body. Built on the two clients above rather than on a repository,
	// because `edges` belongs to graph-grpc-service.
	Mentions              MentionsInterface
	SocialgraphFollowGRPC services.SocialgraphFollowGRPCInterface
	SocialgraphGRPC       services.SocialgraphGRPCInterface
}
