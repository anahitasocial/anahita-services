package main

import (
	"linkpreview/cmd/api/handlers"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
	setViewer echo.MiddlewareFunc
}

func NewRoutes(handler *handlers.Handlers, authorize, setViewer echo.MiddlewareFunc) *Routes {
	return &Routes{handler: handler, authorize: authorize, setViewer: setViewer}
}

func (r *Routes) Init(e *echo.Echo) {
	previews := e.Group("link-previews")

	// Asking for a preview makes this service fetch a page: somebody
	// signed in only.
	previews.GET("", r.handler.Previews.Read, r.authorize)
	previews.GET("/", r.handler.Previews.Read, r.authorize)

	// A preview's picture is read by whoever reads the note it is on, a
	// visitor included where the installation lets visitors read.
	previews.GET("/image/:name", r.handler.Previews.Image, r.setViewer)
}
