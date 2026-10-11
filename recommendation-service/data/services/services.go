package services

import (
	"recommendation/config"

	"anahita-libs/data/services"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:    services.NewActorsGRPC(config),
		FilesGRPC:     services.NewFilesGRPC(config),
		GraphNodeGRPC: services.NewGraphNodeGRPC(config),
	}
}

type Services struct {
	// Reads the profiles that are suggested, as the viewer is sent them
	// anywhere else.
	ActorsGRPC services.ActorsGRPCInterface
	// Signs the addresses of their pictures.
	FilesGRPC services.FilesGRPCInterface
	// Says which of the profiles found the viewer may see, and reads what a
	// reason names: a hashtag, the people who follow somebody.
	GraphNodeGRPC services.GraphNodeGRPCInterface
}
