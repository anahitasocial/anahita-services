package main

import (
	"photo/cmd/api/handlers"

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
	// photos — the read routes are public but still identify the caller
	// when a token is present, so the viewer drives access filtering.
	// Everything else requires a valid OAuth access token.
	photos := e.Group("photos")
	photos.GET("/", r.handler.Photo.Browse, r.setViewer)
	photos.GET("/:owner_id/", r.handler.Photo.Browse, r.setViewer)
	photos.GET("/:id", r.handler.Photo.Read, r.setViewer)
	photos.PATCH("/:id", r.handler.Photo.Edit, r.authorize)
	photos.POST("/:owner_id/", r.handler.Photo.Add, r.authorize)
	photos.DELETE("/:id", r.handler.Photo.Delete, r.authorize)

	// photo access
	photos.PATCH("/:id/access", r.handler.Photo.EditAccess, r.authorize)

	// photo comment status
	photos.PATCH("/:id/commentstatus", r.handler.Photo.EditCommentStatus, r.authorize)
}
