package services

import (
	"anahita-libs/data/services"
	"auth/config"
)

func NewServices(config *config.Config) *Services {
	return &Services{
		FilesGRPC:       services.NewFilesGRPC(config),
		MailGRPC:        services.NewMailGRPC(config),
		ActorAdminsGRPC: services.NewActorAdminsGRPC(config),
		ActorsGRPC:      services.NewActorsGRPC(config),
	}
}

type Services struct {
	FilesGRPC services.FilesGRPCInterface
	MailGRPC  services.MailGRPCInterface

	// ActorAdminsGRPC answers "who administers this group", which is the only
	// question auth-service asks graph-grpc. It is here rather than in
	// notification-service because auth-service is the only service that can
	// send mail AND the only one that can turn an actor id into an email
	// address — and keeping both in one place is what stops the mail path
	// spreading into the general notification fan-out.
	ActorAdminsGRPC services.ActorAdminsGRPCInterface

	// ActorsGRPC answers "what is this group called". Used only to put a name
	// and a shareable link in the lifecycle mail — "a group you administer was
	// deleted" without saying which is not a usable notification.
	//
	// Read still resolves a soft-deleted group, which is what makes this work
	// at all: the notification goes out precisely when the group has just been
	// marked deleted.
	ActorsGRPC services.ActorsGRPCInterface
}
