package main

import (
	"log"

	"anahita-libs/constants"
	"anahita-libs/server"
	"identityimage/cmd/api/handlers"
	"identityimage/cmd/api/permissions"
	"identityimage/config"
	"identityimage/data/services"

	_ "github.com/go-sql-driver/mysql"
)

const (
	webPort = "80"
)

func main() {
	// create config
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	e := server.New(config.IsDev(), constants.ImageRequestBodyMaxSizeLimit)

	// Who asks, from the token: authorize refuses a request with none;
	// setViewer lets it through, for the two addresses anybody may ask,
	// and is what refuses them on a site for members only.
	authorize, setViewer := server.SignIn(config)

	// create handlers
	hEnv := &handlers.Env{
		Config: config,
		Srv:    services.NewServices(config),
		Perms:  permissions.NewPermissions(),
	}
	h := handlers.NewHandlers(hEnv)

	// create routes
	NewRoutes(h, authorize, setViewer).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}
