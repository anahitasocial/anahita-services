package main

import (
	"anahita-libs/data"
	"anahita-libs/federation"
	"anahita-libs/server"
	"anahita-libs/sessions"

	"log"
	"video/data/services"

	_ "github.com/go-sql-driver/mysql"

	"anahita-libs/constants"
	"video/cmd/api/handlers"
	"video/cmd/api/permissions"
	"video/config"
	"video/data/repositories"

	_ "github.com/go-sql-driver/mysql"
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
	// The uploads are kept in the database. Without it nothing can be
	// begun or finished: the service does not start.
	db, err := data.Open(config.GetDSN(), data.Pool{MaxOpen: 10, MaxIdle: 5})
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	hEnv := &handlers.Env{
		Config:  config,
		Perms:   permissions.NewPermissions(),
		Limiter: sessions.NewSession(config),
		Srv:     services.NewServices(config, db),
		// Videos from other networks, copied here when first played.
		Elsewhere: repositories.NewElsewhere(db),
		Refused:   federation.NewBlocks(db).IsBlocked,
	}
	h := handlers.NewHandlers(hEnv)

	// create routes
	NewRoutes(h, authorize, setViewer).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}
