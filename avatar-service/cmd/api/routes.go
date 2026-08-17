package main

import (
	"avatar/cmd/api/handlers"

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
	// avatars — every route requires a valid OAuth access token.
	avatars := e.Group("avatars")
	avatars.Use(r.authorize)
	avatars.POST("/:node_id/", r.handler.Avatar.Add)
	avatars.DELETE("/:node_id/", r.handler.Avatar.Delete)
}
