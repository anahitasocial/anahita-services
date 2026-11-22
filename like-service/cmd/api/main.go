package main

import (
	"log"

	"anahita-libs/constants"
	"anahita-libs/data"
	"anahita-libs/event"
	"anahita-libs/server"
	"like/cmd/api/handlers"
	"like/cmd/api/permissions"
	"like/config"

	"like/data/services"

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

	// connect to the database
	conn := data.ConnectToDB(config.GetDSN())
	if conn == nil {
		log.Panic("can't connect to MySQL!")
	}
	defer conn.Close()

	e := server.New(config.IsDev(), constants.RequestBodyMaxSizeLimit)

	// Who asks, from the token: authorize refuses a request with none,
	// setViewer lets it through.
	authorize, _ := server.SignIn(config)

	// The bus, connected to again whenever it closes the connection.
	publisher, err := event.NewPublisher(config)
	if err != nil {
		log.Panic(err)
	}

	// create permissions env
	// One set of clients, for the permissions and the handlers alike.
	srv := services.NewServices(config)

	pEnv := &permissions.Env{
		Config: config,
		Srv:    srv,
	}

	// create handlers
	hEnv := &handlers.Env{
		Config:    config,
		Srv:       srv,
		Perms:     permissions.NewPermissions(pEnv),
		Publisher: publisher,
	}
	h := handlers.NewHandlers(hEnv)

	// create routes
	NewRoutes(h, authorize).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}
