package services

import (
	"anahita-libs/data/services"
	"group/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:            services.NewActorsGRPC(config),
		ActorAdminsGRPC:       services.NewActorAdminsGRPC(config),
		FilesGRPC:             services.NewFilesGRPC(config),
		SocialgraphFollowGRPC: services.NewSocialgraphFollowGRPC(config),
		SocialgraphGRPC:       services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	ActorsGRPC            services.ActorsGRPCInterface
	ActorAdminsGRPC       services.ActorAdminsGRPCInterface
	FilesGRPC             services.FilesGRPCInterface
	SocialgraphFollowGRPC services.SocialgraphFollowGRPCInterface
	SocialgraphGRPC       services.SocialgraphGRPCInterface
}
