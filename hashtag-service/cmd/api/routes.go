package main

import (
	"hashtag/cmd/api/handlers"

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
	// hashtags — the read routes are public but still identify the
	// caller when a token is present, so the viewer drives access
	// filtering.
	hashtags := e.Group("hashtags")
	hashtags.GET("/_browse/trending", r.handler.Hashtag.BrowseTrending, r.setViewer)
	hashtags.GET("/_browse/ordered_by_degree", r.handler.Hashtag.BrowseOrderedByDegree, r.setViewer)
	hashtags.GET("/", r.handler.Hashtag.Browse, r.setViewer)
	hashtags.GET("/:alias", r.handler.Hashtag.Read, r.setViewer)

	// inbound
	hashtags.GET("/:id/inbounds", r.handler.Inbound.Browse, r.setViewer)

	// admin — requires a valid OAuth access token.
	hashtags.PATCH("/:alias", r.handler.Hashtag.Edit, r.authorize)
	hashtags.DELETE("/:id", r.handler.Hashtag.Delete, r.authorize)
}
