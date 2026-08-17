package services

import (
	"anahita-libs/data/services"
	"auth/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		FilesGRPC: services.NewFilesGRPC(config),
		MailGRPC:  services.NewMailGRPC(config),
	}
}

type Services struct {
	FilesGRPC services.FilesGRPCInterface
	MailGRPC  services.MailGRPCInterface
}
