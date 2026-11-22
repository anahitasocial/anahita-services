package main

import (
	"log"

	"linkpreview/cmd/api/handlers"
	"linkpreview/config"
	"linkpreview/data/services"
	"linkpreview/fetch"

	"anahita-libs/constants"
	"anahita-libs/linkpreviews"
	"anahita-libs/server"
	"anahita-libs/sessions"
)

func main() {
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	e := server.New(config.IsDev(), constants.RequestBodyMaxSizeLimit)

	// Who asks, from the token: authorize refuses a request with none,
	// setViewer lets it through.
	authorize, setViewer := server.SignIn(config)

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

	NewRoutes(h, authorize, setViewer).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}
