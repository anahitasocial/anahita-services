package services

import (
	"abusereport/config"

	"anahita-libs/data/services"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:    services.NewActorsGRPC(config),
		FilesGRPC:     services.NewFilesGRPC(config),
		GraphNodeGRPC: services.NewGraphNodeGRPC(config),
	}
}

type Services struct {
	// ActorsGRPC reads a group's or an event's rules, to check the one a
	// report says was broken.
	ActorsGRPC services.ActorsGRPCInterface
	// FilesGRPC signs the image links on the nodes shown to administrators.
	FilesGRPC services.FilesGRPCInterface
	// GraphNodeGRPC reads what was reported, and answers whether the
	// reporter may see it.
	GraphNodeGRPC services.GraphNodeGRPCInterface
}
