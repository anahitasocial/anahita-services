package services

import (
	"database/sql"

	"anahita-libs/data/services"
	"video/config"
	"video/data/repositories"
)

func NewServices(config *config.Config, db *sql.DB) *Services {
	return &Services{
		ActorsGRPC:      services.NewActorsGRPC(config),
		FilesGRPC:       services.NewFilesGRPC(config),
		Transfers:       services.NewFileTransfersGRPC(config),
		GraphNodeGRPC:   services.NewGraphNodeGRPC(config),
		MediaGRPC:       services.NewMediaGRPC(config),
		SocialgraphGRPC: services.NewSocialgraphGRPC(config),
		Uploads:         repositories.NewUploads(db),
	}
}

type Services struct {
	ActorsGRPC services.ActorsGRPCInterface
	FilesGRPC  services.FilesGRPCInterface
	// Large files: the upload a browser sends straight to storage.
	Transfers services.FileTransfersInterface
	// Asked whether a viewer may see a post, before a file of it is sent.
	GraphNodeGRPC   services.GraphNodeGRPCInterface
	MediaGRPC       services.MediaGRPCInterface
	SocialgraphGRPC services.SocialgraphGRPCInterface
	// The uploads: videos on their way to being posts.
	Uploads repositories.UploadsInterface
}
