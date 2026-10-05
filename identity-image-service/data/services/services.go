package services

import (
	"anahita-libs/data/services"
	"identityimage/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		FilesGRPC:       services.NewFilesGRPC(config),
		ActorsGRPC:      services.NewActorsGRPC(config),
		MediaGRPC:       services.NewMediaGRPC(config),
		GraphNodeGRPC:   services.NewGraphNodeGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	FilesGRPC       services.FilesGRPCInterface
	ActorsGRPC      services.ActorsGRPCInterface
	MediaGRPC       services.MediaGRPCInterface
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
