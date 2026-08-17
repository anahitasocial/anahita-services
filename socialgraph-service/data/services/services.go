package services

import (
	"anahita-libs/data/services"
	"socialgraph/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:            services.NewActorsGRPC(config),
		FilesGRPC:             services.NewFilesGRPC(config),
		GraphGRPC:             services.NewGraphGRPC(config),
		GraphEdgeGRPC:         services.NewGraphEdgeGRPC(config),
		SocialgraphFollowGRPC: services.NewSocialgraphFollowGRPC(config),
		SocialgraphBlockGRPC:  services.NewSocialgraphBlockGRPC(config),
		SocialgraphGRPC:       services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	ActorsGRPC            services.ActorsGRPCInterface
	FilesGRPC             services.FilesGRPCInterface
	GraphGRPC             services.GraphGRPCInterface
	GraphEdgeGRPC         services.GraphEdgeGRPCInterface
	SocialgraphFollowGRPC services.SocialgraphFollowGRPCInterface
	SocialgraphBlockGRPC  services.SocialgraphBlockGRPCInterface
	SocialgraphGRPC       services.SocialgraphGRPCInterface
}
