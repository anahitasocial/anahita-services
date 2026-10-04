package services

import (
	"abusereport/config"

	"anahita-libs/data/services"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		FilesGRPC:     services.NewFilesGRPC(config),
		GraphNodeGRPC: services.NewGraphNodeGRPC(config),
	}
}

type Services struct {
	// FilesGRPC signs the image links on the nodes shown to administrators.
	FilesGRPC services.FilesGRPCInterface
	// GraphNodeGRPC reads what was reported, and answers whether the
	// reporter may see it.
	GraphNodeGRPC services.GraphNodeGRPCInterface
}
