package main

import (
	"identityimage/cmd/api/handlers"

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
	// Each kind of image keeps the address it had when it was a service of
	// its own, and the gateway sends both here. Every route needs a valid
	// OAuth access token; whether that person may change this node is
	// decided in the handler.
	// A public profile's pictures at an address that does not change, for
	// other servers to keep: by the node's UUID, answered with a redirect to
	// a freshly signed address. No token is needed, and whoever is asking
	// makes no difference: see handlers/public.go for what is answered for.
	e.GET("/avatars/:uuid/:size", r.handler.Image.PublicAvatar, r.setViewer)
	e.GET("/covers/:uuid/:size", r.handler.Image.PublicCover, r.setViewer)

	avatars := e.Group("avatars")
	avatars.Use(r.authorize)
	avatars.POST("/:node_id/", r.handler.Image.SetAvatar)
	avatars.DELETE("/:node_id/", r.handler.Image.RemoveAvatar)

	covers := e.Group("covers")
	covers.Use(r.authorize)
	covers.POST("/:node_id/", r.handler.Image.SetCover)
	covers.DELETE("/:node_id/", r.handler.Image.RemoveCover)
}
