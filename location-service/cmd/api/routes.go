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

	// inbound
	//
	// NOTE: Add and Delete are unauthenticated. This preserves the
	// behaviour of the previous route table, where both were registered
	// above the group-level Authorize call and so never picked it up.
	// That looks unintended for write routes — switch them to
	// r.authorize if it was.
	locations.GET("/:id/inbounds", r.handler.Inbound.Browse, r.setViewer)
	locations.POST("/:id/inbounds/", r.handler.Inbound.Add, r.setViewer)
	locations.DELETE("/:id/inbounds/:source_id", r.handler.Inbound.Delete, r.setViewer)

	// admin — requires a valid OAuth access token.
	locations.PATCH("/:id", r.handler.Location.Edit, r.authorize)
	locations.POST("/", r.handler.Location.Add, r.authorize)
	locations.DELETE("/:id", r.handler.Location.Delete, r.authorize)
}
