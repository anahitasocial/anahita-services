package services

import (
	"anahita-libs/data/services"
	"photo/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:      services.NewActorsGRPC(config),
		FilesGRPC:       services.NewFilesGRPC(config),
		Transfers:       services.NewFileTransfersGRPC(config),
		GraphNodeGRPC:   services.NewGraphNodeGRPC(config),
		MediaGRPC:       services.NewMediaGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	ActorsGRPC services.ActorsGRPCInterface
	FilesGRPC  services.FilesGRPCInterface
	// Signs the address a photo is downloaded from, and says which of its
	// copies there are.
	Transfers services.FileTransfersInterface
	// Asked whether a viewer may see a post, before a file of it is sent.
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	MediaGRPC       services.MediaGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
