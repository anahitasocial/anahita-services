package main

import (
	"activitypub/cmd/api/handlers"

	"anahita-libs/apidoc"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
	setViewer echo.MiddlewareFunc
}

func NewRoutes(
	handler *handlers.Handlers,
	authorize echo.MiddlewareFunc,
	setViewer echo.MiddlewareFunc,
) *Routes {
	return &Routes{
		handler:   handler,
		authorize: authorize,
		setViewer: setViewer,
	}
}

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/activitypub-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("activitypub-service", "The fediverse",
		"This installation on the fediverse, by ActivityPub: what other servers ask of it "+
			"and send to it, a person's choice to be on it, finding somebody elsewhere, and the "+
			"servers it refuses (federation.md). Optional: an installation may not run it.")
	api := doc.On(e)

	// What other servers ask of this installation, and send to it.
	r.serverRoutes(api)

	// --- Asked by people here ---
	// Everything under /activitypub is asked by somebody signed in here, so
	// every route needs a token (r.authorize). Who may do what beyond that
	// is checked in the handlers.
	activitypub := api.Group("activitypub", r.authorize)

	// A person's own choice to be on the fediverse, and moving their
	// account out and in.
	r.accountRoutes(activitypub)

	// Finding somebody on another server, and reading them or their
	// posts' pictures again.
	r.remoteRoutes(activitypub)

	// The servers and the accounts this installation refuses.
	r.refusalRoutes(activitypub)

	return doc
}
