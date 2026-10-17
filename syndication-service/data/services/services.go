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
		Domains:               repositories.NewDomains(db),
		Websites:              repositories.NewWebsites(db),
		Items:                 repositories.NewItems(db),
	}
}

type Services struct {
	// The website is made as a profile in the graph, with whoever proved its
	// domain as its first administrator and first follower, as
	// actor-service makes a group.
	ActorsGRPC            services.ActorsGRPCInterface
	ActorAdminsGRPC       services.ActorAdminsGRPCInterface
	SocialgraphFollowGRPC services.SocialgraphFollowGRPCInterface
	// What this service keeps: the domains people prove, each website's feed,
	// and the items seen in it.
	Domains  repositories.DomainsInterface
	Websites repositories.WebsitesInterface
	Items    repositories.ItemsInterface
}
