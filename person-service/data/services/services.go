package services

import (
	"anahita-libs/data/services"
	"person/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:      services.NewActorsGRPC(config),
		FilesGRPC:       services.NewFilesGRPC(config),
		GraphNodeGRPC:   services.NewGraphNodeGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	ActorsGRPC      services.ActorsGRPCInterface
	FilesGRPC       services.FilesGRPCInterface
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
