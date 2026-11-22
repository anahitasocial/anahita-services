package main

import (
	"log"

	"anahita-libs/constants"
	"anahita-libs/data"
	"anahita-libs/server"
	"search/cmd/api/handlers"
	"search/cmd/api/permissions"
	"search/config"
	"search/data/services"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// create config
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	// connect to the database
	conn := data.ConnectToDB(config.GetDSN())
	if conn == nil {
		log.Panic("can't connect to MySQL!")
	}
	defer conn.Close()

	e := server.New(config.IsDev(), constants.RequestBodyMaxSizeLimit)

	// Who asks, from the token: setViewer lets a request with none
	// through; authorize refuses it, for the viewer's own choices.
	authorize, setViewer := server.SignIn(config)

	// create handlers
	hEnv := &handlers.Env{
		Config: config,
		Perms:  permissions.NewPermissions(),
		Srv:    services.NewServices(config),
	}
	h := handlers.NewHandlers(hEnv)

	// create routes
	NewRoutes(h, setViewer, authorize).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}
