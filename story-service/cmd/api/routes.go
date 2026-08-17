package main

import (
	"story/cmd/api/handlers"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	setViewer echo.MiddlewareFunc
}

func NewRoutes(
	handler *handlers.Handlers,
	setViewer echo.MiddlewareFunc,
) *Routes {
	return &Routes{
		handler:   handler,
		setViewer: setViewer,
	}
}

func (r *Routes) Init(e *echo.Echo) {
	// storys — public, but still identifies the caller when a token is
	// present so the viewer drives access filtering.
	storys := e.Group("stories")
	storys.GET("/", r.handler.Story.Browse, r.setViewer)
}
