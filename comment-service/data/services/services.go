package services

import (
	"anahita-libs/data/services"
	"comment/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		FilesGRPC:       services.NewFilesGRPC(config),
		GraphNodeGRPC:   services.NewGraphNodeGRPC(config),
		LikesGRPC:       services.NewLikesGRPC(config),
		MediaGRPC:       services.NewMediaGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	FilesGRPC       services.FilesGRPCInterface
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	LikesGRPC       services.LikesGRPCInterface
	MediaGRPC       services.MediaGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
