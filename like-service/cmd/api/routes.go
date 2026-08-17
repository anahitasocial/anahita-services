package main

import (
	"like/cmd/api/handlers"

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
	// likes — every route requires a valid OAuth access token.
	likes := e.Group("likes")
	likes.Use(r.authorize)
	likes.GET("/:node_id/", r.handler.Like.Browse)
	likes.POST("/:node_id/", r.handler.Like.Add)
	likes.DELETE("/:node_id/", r.handler.Like.Delete)
}
