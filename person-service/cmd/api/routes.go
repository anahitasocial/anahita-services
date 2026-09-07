package main

import (
	"person/cmd/api/handlers"

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

func (r *Routes) Init(e *echo.Echo) {
	// people — Browse and Read are public but still identify the caller
	// when a token is present, so the viewer drives access filtering.
	// Everything else requires a valid OAuth access token.
	people := e.Group("people")
	people.GET("/", r.handler.Person.Browse, r.setViewer)
	people.GET("/:username", r.handler.Person.Read, r.setViewer)
	people.PATCH("/:id", r.handler.Person.Edit, r.authorize)
	// :id, not :username. The client sends node.id here (api/create.js
	// deleteItem builds /people/{id}), so this route matched the username
	// against a number and answered 404 to every delete — the flow could never
	// have run. PATCH above already keys on the id, and group-service keys its
	// delete on the id too, so this is the consistent shape as well as the
	// working one.
	people.DELETE("/:id", r.handler.Person.Delete, r.authorize)

	// Edit access
	people.PATCH("/:id/access", r.handler.Person.EditAccess, r.authorize)

	// Edit features
	people.PATCH("/:id/features", r.handler.Person.EditFeatures, r.authorize)
}
