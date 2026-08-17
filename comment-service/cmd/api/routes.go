package main

import (
	"comment/cmd/api/handlers"

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
	// comments — Browse is public but still identifies the caller when a
	// token is present, so the viewer drives access filtering. The write
	// routes require a valid access token.
	notes := e.Group("comments/:parent_id")
	notes.GET("/", r.handler.Comment.Browse, r.setViewer)
	// notes.GET("/:id", r.handler.Comment.Read, r.setViewer)
	notes.PATCH("/:id", r.handler.Comment.Edit, r.authorize)
	notes.POST("/", r.handler.Comment.Add, r.authorize)
	notes.DELETE("/:id", r.handler.Comment.Delete, r.authorize)
}
