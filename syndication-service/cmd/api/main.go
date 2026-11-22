package main

import (
	"anahita-libs/data"
	"anahita-libs/server"
	"log"

	"syndication/cmd/api/handlers"
	"syndication/config"
	"syndication/data/services"
	"syndication/feeds"

	"anahita-libs/constants"
	"anahita-libs/domains"
	"anahita-libs/sessions"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}
	e := server.New(config.IsDev(), constants.RequestBodyMaxSizeLimit)

	// Who asks, from the token: authorize refuses a request with none,
	// setViewer lets it through.
	authorize, setViewer := server.SignIn(config)

	// The channels, their feeds' items and the domains proved for them are
	// kept in the database: without it nothing can be connected.
	db, err := data.Open(config.GetDSN(), data.Pool{MaxOpen: 10, MaxIdle: 5})
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	h := handlers.NewHandlers(&handlers.Env{
		Config:  config,
		Srv:     services.NewServices(config, db),
		Feeds:   feeds.NewReader(),
		Domains: domains.NewChecker(),
		Limiter: sessions.NewSession(config),
	})

	NewRoutes(h, authorize, setViewer).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}
