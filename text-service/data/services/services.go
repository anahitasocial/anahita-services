package services

import (
	"anahita-libs/data/services"
	"text/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:      services.NewActorsGRPC(config),
		FilesGRPC:       services.NewFilesGRPC(config),
		MediaGRPC:       services.NewMediaGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
		GraphNodeGRPC:   services.NewGraphNodeGRPC(config),
	}
}

type Services struct {
	ActorsGRPC      services.ActorsGRPCInterface
	FilesGRPC       services.FilesGRPCInterface
	MediaGRPC       services.MediaGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
	// Reads a post as it is, whoever is asking: for the one question of who
	// wrote a quoted post, when its author takes it out of a quote.
	GraphNodeGRPC services.GraphNodeGRPCInterface
}
