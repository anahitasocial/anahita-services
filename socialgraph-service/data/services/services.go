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
		SocialgraphMuteGRPC:   services.NewSocialgraphMuteGRPC(config),
		// The servers each person blocked for themselves (roadmap row 152).
		SocialgraphServerBlockGRPC: services.NewSocialgraphServerBlockGRPC(config),
		SocialgraphGRPC:            services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	ActorsGRPC                 services.ActorsGRPCInterface
	FilesGRPC                  services.FilesGRPCInterface
	GraphGRPC                  services.GraphGRPCInterface
	GraphEdgeGRPC              services.GraphEdgeGRPCInterface
	SocialgraphFollowGRPC      services.SocialgraphFollowGRPCInterface
	SocialgraphBlockGRPC       services.SocialgraphBlockGRPCInterface
	SocialgraphMuteGRPC        services.SocialgraphMuteGRPCInterface
	SocialgraphServerBlockGRPC services.SocialgraphServerBlockGRPCInterface
	SocialgraphGRPC            services.SocialgraphGRPCInterface
}
