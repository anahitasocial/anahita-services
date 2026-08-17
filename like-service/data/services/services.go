package services

import (
	"anahita-libs/data/services"
	"like/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:      services.NewActorsGRPC(config),
		FilesGRPC:       services.NewFilesGRPC(config),
		GraphGRPC:       services.NewGraphGRPC(config),
		GraphNodeGRPC:   services.NewGraphNodeGRPC(config),
		LikesGRPC:       services.NewLikesGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	ActorsGRPC      services.ActorsGRPCInterface
	FilesGRPC       services.FilesGRPCInterface
	GraphGRPC       services.GraphGRPCInterface
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	LikesGRPC       services.LikesGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
