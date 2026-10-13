package main

import (
	"atproto/cmd/api/handlers"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
}

func NewRoutes(handler *handlers.Handlers, authorize echo.MiddlewareFunc) *Routes {
	return &Routes{handler: handler, authorize: authorize}
}

func (r *Routes) Init(e *echo.Echo) {
	atproto := e.Group("atproto")
	atproto.Use(r.authorize)
	// Finding somebody on Bluesky.
	atproto.GET("/lookup", r.handler.Lookup)
}
