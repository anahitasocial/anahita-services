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
	feeds.GET("/actor/:actor_id/", r.handler.FeedActor.Browse, r.setViewer)

	// Custom feeds. Making, changing and keeping one need somebody signed
	// in; reading one does not, when it is public. The fixed paths are
	// matched before ":id".
	custom := r.handler.CustomFeed
	feeds.POST("/", custom.Add, r.authorize)
	feeds.GET("/mine", custom.Mine, r.authorize)
	feeds.GET("/pinned", custom.Pinned, r.authorize)
	feeds.PUT("/pins", custom.SetPins, r.authorize)
	feeds.POST("/preview", custom.Preview, r.authorize)
	feeds.GET("/:id", custom.Read, r.setViewer)
	feeds.PATCH("/:id", custom.Edit, r.authorize)
	feeds.DELETE("/:id", custom.Delete, r.authorize)
	feeds.GET("/:id/items", custom.Items, r.setViewer)
	feeds.POST("/:id/copy", custom.Copy, r.authorize)
	feeds.PUT("/:id/pin", custom.Pin, r.authorize)
	feeds.DELETE("/:id/pin", custom.Unpin, r.authorize)

	// reposts — every route requires a valid OAuth access token.
	reposts := e.Group("reposts/")
	reposts.Use(r.authorize)
	reposts.GET(":parent_id/", r.handler.Repost.Browse)
	reposts.POST(":parent_id/", r.handler.Repost.Add)
	reposts.DELETE(":parent_id/", r.handler.Repost.Delete)
}
