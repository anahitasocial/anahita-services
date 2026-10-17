package services

import (
	"database/sql"

	"pulse/config"
	"pulse/data/repositories"

	"anahita-libs/data/services"
)

func NewServices(config *config.Config, db *sql.DB) *Services {
	return &Services{
		ActorsGRPC:            services.NewActorsGRPC(config),
		ActorAdminsGRPC:       services.NewActorAdminsGRPC(config),
		SocialgraphFollowGRPC: services.NewSocialgraphFollowGRPC(config),
		Domains:               repositories.NewDomains(db),
		Pulses:                repositories.NewPulses(db),
		Items:                 repositories.NewItems(db),
	}
}

type Services struct {
	// The Pulse is made as a profile in the graph, with whoever proved its
	// domain as its first administrator and first follower, as
	// actor-service makes a group.
	ActorsGRPC            services.ActorsGRPCInterface
	ActorAdminsGRPC       services.ActorAdminsGRPCInterface
	SocialgraphFollowGRPC services.SocialgraphFollowGRPCInterface
	// What this service keeps: the domains people prove, each Pulse's feed,
	// and the items seen in it.
	Domains repositories.DomainsInterface
	Pulses  repositories.PulsesInterface
	Items   repositories.ItemsInterface
}
