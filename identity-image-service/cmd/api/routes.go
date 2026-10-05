package main

import (
	"identityimage/cmd/api/handlers"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
}

func NewRoutes(
	handler *handlers.Handlers,
	authorize echo.MiddlewareFunc,
) *Routes {
	return &Routes{
		handler:   handler,
		authorize: authorize,
	}
}

func (r *Routes) Init(e *echo.Echo) {
	// Each kind of image keeps the address it had when it was a service of
	// its own, and the gateway sends both here. Every route needs a valid
	// OAuth access token; whether that person may change this node is
	// decided in the handler.
	avatars := e.Group("avatars")
	avatars.Use(r.authorize)
	avatars.POST("/:node_id/", r.handler.Image.SetAvatar)
	avatars.DELETE("/:node_id/", r.handler.Image.RemoveAvatar)

	covers := e.Group("covers")
	covers.Use(r.authorize)
	covers.POST("/:node_id/", r.handler.Image.SetCover)
	covers.DELETE("/:node_id/", r.handler.Image.RemoveCover)
}
