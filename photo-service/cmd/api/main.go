package main

import (
	"anahita-libs/server"
	"anahita-libs/sessions"
	"log"
	"photo/data/services"

	"anahita-libs/constants"
	"photo/cmd/api/handlers"
	"photo/cmd/api/permissions"
	"photo/config"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// create config
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	e := server.New(config.IsDev(), constants.ImageRequestBodyMaxSizeLimit)

	// Who asks, from the token: authorize refuses a request with none,
	// setViewer lets it through.
	authorize, setViewer := server.SignIn(config)

	// create handlers
	hEnv := &handlers.Env{
		Config:  config,
		Perms:   permissions.NewPermissions(),
		Limiter: sessions.NewSession(config),
		Srv:     services.NewServices(config),
	}
	h := handlers.NewHandlers(hEnv)

	// create routes
	NewRoutes(h, authorize, setViewer).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}
