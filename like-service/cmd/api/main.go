package main

import (
	"fmt"
	"log"
	"net/http"

	"anahita-libs/constants"
	"anahita-libs/data"
	"anahita-libs/event"
	an_mw "anahita-libs/middleware"
	"anahita-libs/validators"
	"like/cmd/api/handlers"
	"like/cmd/api/permissions"
	"like/config"

	"like/data/services"

	"github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	webPort = "80"
)

func main() {
	e := echo.New()
	e.IPExtractor = an_mw.ClientIPExtractor()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit(constants.RequestBodyMaxSizeLimit))

	// set custom validator
	e.Validator = validators.NewRequestValidator(validator.New())

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

	if config.IsDev() {
		e.Debug = true
	}

	// JWKS client. One per process. Public keys are fetched from
	// auth-service's JWKS endpoint and cached, so access tokens are
	// verified locally on every request without a roundtrip.
	jwks := an_mw.NewJWKSClient(config.GetOAuthJWKSURL())

	// Pre-built middleware for use in routes. Authorize rejects
	// unauthenticated requests; the viewer it builds from the token
	// claims is read by handlers via an_mw.GetViewer.
	authorize := an_mw.Authorize(config, jwks)

	// create amqp connection
	amqpConn, err := event.ConnectToAMQP(config)
	if err != nil {
		log.Panic(err)
	}

	// create event publisher
	publisher, err := event.NewEventPublisher(amqpConn)
	if err != nil {
		log.Panic(err)
	}

	// create permissions env
	pEnv := &permissions.Env{
		Config: config,
		Srv:    services.NewServices(config),
	}

	// create handlers
	hEnv := &handlers.Env{
		Config:    config,
		Srv:       services.NewServices(config),
		Perms:     permissions.NewPermissions(pEnv),
		Publisher: publisher,
	}
	h := handlers.NewHandlers(hEnv)

	// heartbeat
	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	// create routes
	NewRoutes(h, authorize).Init(e)

	// start server
	err = e.Start(fmt.Sprintf(":%v", webPort))
	if err != nil {
		e.Logger.Fatal(err)
	}
}
