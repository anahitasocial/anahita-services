package main

import (
	"log"

	"anahita-libs/constants"
	"anahita-libs/linkpreviews"
	"anahita-libs/server"
	"anahita-libs/sessions"
	"text/cmd/api/handlers"
	"text/cmd/api/permissions"
	"text/config"
	"text/data/services"

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

	e := server.New(config.IsDev(), constants.RequestBodyMaxSizeLimit)

	// Who asks, from the token: authorize refuses a request with none,
	// setViewer lets it through.
	authorize, setViewer := server.SignIn(config)

	// create handlers
	hEnv := &handlers.Env{
		Config: config,
		Srv:    services.NewServices(config),
		Perms:  permissions.NewPermissions(),
	}
	// Where link-preview-service keeps what it read. Without it notes are
	// saved as they always were, with no preview card.
	// Connected to when a note first needs it, and looked for again while
	// it is away: the cache may still be starting when this does.
	hEnv.Previews = linkpreviews.NewStoreWhenReady(func() (linkpreviews.Cache, error) {
		return sessions.NewCache(config)
	})
	h := handlers.NewHandlers(hEnv)

	// create routes
	NewRoutes(h, authorize, setViewer).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}
