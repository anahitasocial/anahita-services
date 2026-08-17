package services

import (
	"anahita-libs/data/services"
	"notification/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:        services.NewActorsGRPC(config),
		ActorAdminsGRPC:   services.NewActorAdminsGRPC(config),
		FilesGRPC:         services.NewFilesGRPC(config),
		MediaGRPC:         services.NewMediaGRPC(config),
		GraphNodeGRPC:     services.NewGraphNodeGRPC(config),
		SocialgraphGRPC:   services.NewSocialgraphGRPC(config),
		SubscriptionsGRPC: services.NewSubscriptionsGRPC(config),
	}
}

type Services struct {
	ActorsGRPC        services.ActorsGRPCInterface
	ActorAdminsGRPC   services.ActorAdminsGRPCInterface
	FilesGRPC         services.FilesGRPCInterface
	MediaGRPC         services.MediaGRPCInterface
	GraphNodeGRPC     services.GraphNodeGRPCInterface
	SocialgraphGRPC   services.SocialgraphGRPCInterface
	SubscriptionsGRPC services.SubscriptionsGRPCInterface
}
