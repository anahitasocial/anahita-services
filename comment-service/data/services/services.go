package services

import (
	"anahita-libs/data/services"
	"comment/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		ActorsGRPC:      services.NewActorsGRPC(config),
		FilesGRPC:       services.NewFilesGRPC(config),
		GraphNodeGRPC:   services.NewGraphNodeGRPC(config),
		LikesGRPC:       services.NewLikesGRPC(config),
		MediaGRPC:       services.NewMediaGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
	}
}

type Services struct {
	// Loads the owner of the post being commented on. The medium read
	// carries only the owner's id, and the comment permission reads the
	// owner's access level and its add_permissions — which only the actor
	// read returns with the actor_features defaults merged in.
	ActorsGRPC      services.ActorsGRPCInterface
	FilesGRPC       services.FilesGRPCInterface
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	LikesGRPC       services.LikesGRPCInterface
	MediaGRPC       services.MediaGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
}
