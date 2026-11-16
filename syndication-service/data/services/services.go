package services

import (
	"database/sql"

	"syndication/config"
	"syndication/data/repositories"

	"anahita-libs/data/services"
)

func NewServices(config *config.Config, db *sql.DB) *Services {
	return &Services{
		ActorsGRPC:            services.NewActorsGRPC(config),
		ActorAdminsGRPC:       services.NewActorAdminsGRPC(config),
		SocialgraphFollowGRPC: services.NewSocialgraphFollowGRPC(config),
		GraphGRPC:             services.NewGraphGRPC(config),
		Domains:               repositories.NewDomains(db),
		Channels:              repositories.NewChannels(db),
		Items:                 repositories.NewItems(db),
	}
}

type Services struct {
	// The channel is made as a profile in the graph, with whoever proved its
	// domain as its first administrator and first follower, as
	// actor-service makes a group.
	ActorsGRPC            services.ActorsGRPCInterface
	ActorAdminsGRPC       services.ActorAdminsGRPCInterface
	SocialgraphFollowGRPC services.SocialgraphFollowGRPCInterface
	// What the instance offers (roadmap row 139): a channel is not made
	// where it does not offer channels.
	GraphGRPC services.GraphGRPCInterface
	// What this service keeps: the domains people prove, each channel's feed,
	// and the items seen in it.
	Domains  repositories.DomainsInterface
	Channels repositories.ChannelsInterface
	Items    repositories.ItemsInterface
}
