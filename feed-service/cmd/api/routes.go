package main

import (
	"feed/cmd/api/handlers"

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
	// feeds — public, but still identify the caller when a token is
	// present so the viewer drives access filtering.
	feeds := e.Group("feeds")
	feeds.GET("/leaders/", r.handler.FeedLeaders.Browse, r.setViewer)
	// Who the viewer follows that posted lately. For somebody signed in:
	// it is about the people they follow.
	feeds.GET("/leaders/active", r.handler.FeedLeaders.Active, r.authorize)
	feeds.GET("/actor/:actor_id/", r.handler.FeedActor.Browse, r.setViewer)

	// reposts — every route requires a valid OAuth access token.
	reposts := e.Group("reposts/")
	reposts.Use(r.authorize)
	reposts.GET(":parent_id/", r.handler.Repost.Browse)
	reposts.POST(":parent_id/", r.handler.Repost.Add)
	reposts.DELETE(":parent_id/", r.handler.Repost.Delete)
}
