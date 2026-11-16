package services

import (
	"recommendation/config"

	"anahita-libs/data/services"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:    services.NewActorsGRPC(config),
		EventsGRPC:    services.NewEventsGRPC(config),
		FilesGRPC:     services.NewFilesGRPC(config),
		GraphNodeGRPC: services.NewGraphNodeGRPC(config),
		// How the viewer stands with the people a reason names, for who
		// may see whom they follow (roadmap row 151).
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	// Reads the profiles that are suggested, as the viewer is sent them
	// anywhere else.
	ActorsGRPC services.ActorsGRPCInterface
	// Reads when a suggested event is, and how many are going.
	EventsGRPC services.EventsGRPCInterface
	// Signs the addresses of their pictures.
	FilesGRPC services.FilesGRPCInterface
	// Says which of the profiles found the viewer may see, and reads what a
	// reason names: a hashtag, the people who follow somebody.
	GraphNodeGRPC services.GraphNodeGRPCInterface
	// Reads how the viewer stands with the people a reason names.
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
