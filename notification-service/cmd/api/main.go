package main

import (
	"log"

	"anahita-libs/constants"
	"anahita-libs/data"
	"anahita-libs/server"
	"notification/cmd/api/handlers"
	"notification/cmd/api/permissions"
	"notification/config"
	"notification/data/repositories"
	"notification/data/services"

	_ "github.com/go-sql-driver/mysql"
)

const (
	webPort  = "80"
	gRpcPort = "50051"
)

func main() {
	var err error

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

	// create handlers
	hEnv := &handlers.Env{
		Config: config,
		Repos:  repositories.NewRepositories(conn),
		Srv:    services.NewServices(config),
		Perms:  permissions.NewPermissions(),
	}
	h := handlers.NewHandlers(hEnv)

	// Start the event consumer
	go func() {
		if err = startEventListerner(h, config); err != nil {
			log.Panic(err)
		}
	}()

	// Read notifications past keeping go (NOTIFICATIONS_KEEP_DAYS).
	go keepNotifications(hEnv.Repos.Notifications, config.KeepDays)

	// create routes
	NewRoutes(h, authorize).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}
