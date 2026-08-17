package services

import (
	"anahita-libs/data/services"
	"avatar/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		FilesGRPC:       services.NewFilesGRPC(config),
		ActorsGRPC:      services.NewActorsGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	FilesGRPC       services.FilesGRPCInterface
	ActorsGRPC      services.ActorsGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
