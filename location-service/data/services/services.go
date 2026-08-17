package services

import (
	"anahita-libs/data/services"
	"anahita-libs/geocoding"
	"location/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		FilesGRPC:     services.NewFilesGRPC(config),
		GraphGRPC:     services.NewGraphGRPC(config),
		GraphEdgeGRPC: services.NewGraphEdgeGRPC(config),
		GraphNodeGRPC: services.NewGraphNodeGRPC(config),
		Location:      geocoding.NewLocationService(config),
	}
}

type Services struct {
	FilesGRPC     services.FilesGRPCInterface
	GraphGRPC     services.GraphGRPCInterface
	GraphEdgeGRPC services.GraphEdgeGRPCInterface
	GraphNodeGRPC services.GraphNodeGRPCInterface
	Location      *geocoding.LocationService
}
