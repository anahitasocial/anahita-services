package main

import (
	"cover/cmd/api/handlers"

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
	// covers — every route requires a valid OAuth access token.
	covers := e.Group("covers")
	covers.Use(r.authorize)
	covers.POST("/:node_id/", r.handler.Cover.Add)
	covers.DELETE("/:node_id/", r.handler.Cover.Delete)
}
