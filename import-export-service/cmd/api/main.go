package main

import (
	"anahita-libs/data"
	"anahita-libs/server"
	"log"
	"net/http"

	"importexport/cmd/api/handlers"
	"importexport/config"
	"importexport/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/services"
	"anahita-libs/sessions"

	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	e := server.New(config.IsDev(), "")
	// Every request kept small but an import's file, which has its own
	// limit on its route (UploadLimit).
	e.Use(middleware.BodyLimitWithConfig(middleware.BodyLimitConfig{
		Limit: constants.RequestBodyMaxSizeLimit,
		Skipper: func(c echo.Context) bool {
			return c.Request().Method == http.MethodPost && c.Path() == "/imports"
		},
	}))

	authorize, _ := server.SignIn(config)

	db, err := data.Open(config.GetDSN(), data.Pool{MaxOpen: 10, MaxIdle: 5})
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	env := &handlers.Env{
		Config:    config,
		Exports:   repositories.NewExports(db),
		Imports:   repositories.NewImports(db),
		Transfers: services.NewFileTransfersGRPC(config),
		Files:     services.NewFilesGRPC(config),
		Session:   stepUpStore(config),
	}

	NewRoutes(env, authorize).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}

// stepUpStore connects to Valkey for the marker that says somebody has just
// proved who they are, or returns nil. With nil, asking for an export is
// refused: a gate that cannot check does not open.
func stepUpStore(cfg *config.Config) sessions.KeyValueInterface {
	store, err := sessions.NewKeyValueStore(cfg)
	if err != nil {
		log.Printf("step-up: no Valkey (%v): asking for an export will be refused until it is reachable", err)
		return nil
	}
	return store
}
