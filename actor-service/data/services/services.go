package services

import (
	"actor/config"
	"anahita-libs/data/services"
	"anahita-libs/geocoding"
	"context"
)

// Geocoder finds where an address is on a map.
type Geocoder interface {
	Geocode(ctx context.Context, address string) (*geocoding.Location, error)
}

func NewServices(config *config.Config) *Services {
	nodes := services.NewGraphNodeGRPC(config)
	edges := services.NewGraphEdgeGRPC(config)

	// Left nil with every geocoder switched off, and events are then saved
	// without a map. Assigned only when there is one: a nil pointer put in
	// the interface would not compare equal to nil.
	var geocoder Geocoder
	if found := geocoding.NewOptionalLocationService(config); found != nil {
		geocoder = found
	}

	return &Services{
		Geocoder:        geocoder,
		ActorsGRPC:      services.NewActorsGRPC(config),
		ActorAdminsGRPC: services.NewActorAdminsGRPC(config),
		EventsGRPC:      services.NewEventsGRPC(config),
		// Who is going to an event: the event's followers, by their answer.
		GraphGRPC:             services.NewGraphGRPC(config),
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
	// Finds an event's address on a map, when whoever makes it asks.
	Geocoder      Geocoder
	GraphGRPC     services.GraphGRPCInterface
	FilesGRPC     services.FilesGRPCInterface
	GraphEdgeGRPC services.GraphEdgeGRPCInterface
	GraphNodeGRPC services.GraphNodeGRPCInterface
	// Mentions keeps a node's mention edges in step with the @handles in its
	// body. Built on the two clients above rather than on a repository,
	// because `edges` belongs to graph-grpc-service.
	Mentions              MentionsInterface
	SocialgraphFollowGRPC services.SocialgraphFollowGRPCInterface
	SocialgraphGRPC       services.SocialgraphGRPCInterface
}
