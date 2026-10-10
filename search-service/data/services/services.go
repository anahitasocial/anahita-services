package services

import (
	"anahita-libs/data/services"
	"search/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		FilesGRPC:     services.NewFilesGRPC(config),
		GraphNodeGRPC: services.NewGraphNodeGRPC(config),
		SearchGRPC:    services.NewSearchGRPC(config),
		// How each person or group found stands with the viewer.
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	FilesGRPC       services.FilesGRPCInterface
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	SearchGRPC      services.SearchGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
