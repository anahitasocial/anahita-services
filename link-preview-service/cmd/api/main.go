package main

import (
	"fmt"
	"log"
	"net/http"

	"linkpreview/cmd/api/handlers"
	"linkpreview/config"
	"linkpreview/data/services"
	"linkpreview/fetch"

	"anahita-libs/constants"
	"anahita-libs/linkpreviews"
	an_mw "anahita-libs/middleware"
	"anahita-libs/sessions"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
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
	setViewer := an_mw.SetViewer(config, jwks)

	// Where previews are kept while they are fresh. Without it every asking
	// would fetch the page again, and a note could not be given the preview
	// its writer was shown: the service does not start.
	cache, err := sessions.NewCache(config)
	if err != nil {
		log.Fatalf("could not reach the cache: %v", err)
	}

	hEnv := &handlers.Env{
		Config:  config,
		Srv:     services.NewServices(config),
		Fetcher: fetch.New(),
		Store:   linkpreviews.NewStore(cache),
		Limiter: sessions.NewSession(config),
	}
	h := handlers.NewHandlers(hEnv)

	// Fail at start-up, not at the first report, if the installation's own
	// reasons file is unusable. The service still runs, on the built-in
	// list, and the log has already said why.

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	NewRoutes(h, authorize, setViewer).Init(e)

	if err := e.Start(fmt.Sprintf(":%v", webPort)); err != nil {
		e.Logger.Fatal(err)
	}
}
