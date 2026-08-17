package services

import (
	"anahita-libs/data/services"
	"photo/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:      services.NewActorsGRPC(config),
		FilesGRPC:       services.NewFilesGRPC(config),
		MediaGRPC:       services.NewMediaGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	ActorsGRPC      services.ActorsGRPCInterface
	FilesGRPC       services.FilesGRPCInterface
	MediaGRPC       services.MediaGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
