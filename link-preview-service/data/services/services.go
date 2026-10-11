package services

import (
	"linkpreview/config"

	"anahita-libs/data/services"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		FilesGRPC: services.NewFilesGRPC(config),
	}
}

type Services struct {
	// Keeps a preview's picture in the installation's own storage, and
	// signs the address it is read from.
	FilesGRPC services.FilesGRPCInterface
}
