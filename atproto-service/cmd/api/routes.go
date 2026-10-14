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
	// The pictures of a post from Bluesky, copied here again after they
	// were removed for its age.
	atproto.POST("/posts/:id/pictures", r.handler.RestorePictures)
	// A person's own Bluesky account, connected or not.
	atproto.GET("/settings", r.handler.ReadSettings)
	atproto.PUT("/settings", r.handler.EditSettings)
	atproto.DELETE("/settings", r.handler.DeleteSettings)
	// Following here whoever they follow on Bluesky already.
	atproto.GET("/settings/import-follows", r.handler.ImportPreview)
	atproto.POST("/settings/import-follows", r.handler.ImportFollows)
}
