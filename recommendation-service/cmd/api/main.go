package main

import (
	"anahita-libs/data"
	"anahita-libs/server"
	"log"

	"recommendation/cmd/api/handlers"
	"recommendation/config"
	"recommendation/data/repositories"
	"recommendation/data/services"

	"anahita-libs/constants"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	e := server.New(config.IsDev(), constants.RequestBodyMaxSizeLimit)

	db, err := data.Open(config.GetDSN(), data.Pool{MaxOpen: 10, MaxIdle: 5})
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	// Who asks, from the token: authorize refuses a request with none,
	// setViewer lets it through.
	authorize, _ := server.SignIn(config)

	hEnv := &handlers.Env{
		Config: config,
		Repos:  repositories.NewRepositories(db),
		Srv:    services.NewServices(config),
	}
	h := handlers.NewHandlers(hEnv)

	// Fail at start-up, not at the first report, if the installation's own
	// reasons file is unusable. The service still runs, on the built-in
	// list, and the log has already said why.

	go func() {
		if err := startEventListener(h, config); err != nil {
			log.Panic(err)
		}
	}()

	NewRoutes(h, authorize).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}
