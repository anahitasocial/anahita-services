package main

import (
	"location/cmd/api/handlers"

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
	// locations — the read routes are public but still identify the
	// caller when a token is present, so the viewer drives access
	// filtering.
	locations := e.Group("locations")
	locations.GET("/_browse/trending", r.handler.Location.BrowseTrending, r.setViewer)
	locations.GET("/_browse/ordered_by_degree", r.handler.Location.BrowseOrderedByDegree, r.setViewer)
	locations.GET("/", r.handler.Location.Browse, r.setViewer)
	locations.GET("/:id", r.handler.Location.Read, r.setViewer)

	// inbound — browsing is public like the reads above; tagging and
	// untagging are writes and require a token. They previously sat on
	// setViewer only, which was a leftover from an older route table
	// rather than a decision, and left both open to anonymous callers.
	locations.GET("/:id/inbounds", r.handler.Inbound.Browse, r.setViewer)
	locations.POST("/:id/inbounds/", r.handler.Inbound.Add, r.authorize)
	locations.DELETE("/:id/inbounds/:source_id", r.handler.Inbound.Delete, r.authorize)

	// admin — requires a valid OAuth access token.
	locations.PATCH("/:id", r.handler.Location.Edit, r.authorize)
	locations.POST("/", r.handler.Location.Add, r.authorize)
	locations.DELETE("/:id", r.handler.Location.Delete, r.authorize)
}
