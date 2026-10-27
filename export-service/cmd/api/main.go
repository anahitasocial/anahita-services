package main

import (
	"anahita-libs/data"
	"fmt"
	"log"
	"net/http"

	"export/cmd/api/handlers"
	"export/config"
	"export/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"
	"anahita-libs/sessions"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const webPort = "80"

func main() {
	e := echo.New()
	e.IPExtractor = an_mw.ClientIPExtractor()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit(constants.RequestBodyMaxSizeLimit))

	e.Validator = validators.NewRequestValidator(validator.New())

	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}
	if config.IsDev() {
		e.Debug = true
	}

	jwks := an_mw.NewJWKSClient(config.GetOAuthJWKSURL())
	authorize := an_mw.Authorize(config, jwks)

	db, err := data.Open(config.GetDSN(), data.Pool{MaxOpen: 10, MaxIdle: 5})
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	h := handlers.NewExportsHandler(&handlers.Env{
		Config:    config,
		Exports:   repositories.NewExports(db),
		Transfers: services.NewFileTransfersGRPC(config),
		Files:     services.NewFilesGRPC(config),
		Session:   stepUpStore(config),
	})

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	NewRoutes(h, authorize).Init(e)

	if err := e.Start(fmt.Sprintf(":%v", webPort)); err != nil {
		e.Logger.Fatal(err)
	}
}

// stepUpStore connects to Redis for the marker that says somebody has just
// proved who they are, or returns nil. With nil, asking for an export is
// refused: a gate that cannot check does not open.
func stepUpStore(cfg *config.Config) sessions.KeyValueInterface {
	store, err := sessions.NewKeyValueStore(cfg)
	if err != nil {
		log.Printf("step-up: no Redis (%v): asking for an export will be refused until it is reachable", err)
		return nil
	}
	return store
}
