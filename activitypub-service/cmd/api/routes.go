package main

import (
	"activitypub/cmd/api/handlers"

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
	// What other servers ask. No token, since a server has none: who is
	// answered for is decided in each handler, and it is only ever somebody
	// who chose to be on the fediverse. setViewer is on each because it is
	// what answers for a site that is for members only.
	e.GET("/.well-known/webfinger", r.handler.WebFinger, r.setViewer)
	e.GET("/.well-known/host-meta", r.handler.HostMeta, r.setViewer)

	ap := e.Group("ap")
	ap.GET("/users/:uuid", r.handler.Actor, r.setViewer)
	ap.GET("/users/:uuid/followers", r.handler.Followers, r.setViewer)
	ap.GET("/users/:uuid/following", r.handler.Following, r.setViewer)

	// A person's own choice to be on the fediverse.
	activitypub := e.Group("activitypub")
	activitypub.Use(r.authorize)
	activitypub.GET("/settings", r.handler.ReadSettings)
	activitypub.PUT("/settings", r.handler.EditSettings)
}
