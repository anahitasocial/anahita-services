package main

import (
	"socialgraph/cmd/api/handlers"

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
	// socialgraph — followers and leaders are public but still identify
	// the caller when a token is present, so the viewer drives access
	// filtering. Everything else requires a valid OAuth access token.
	socialgraph := e.Group("socialgraph")
	socialgraph.GET("/:actor_id/followers/", r.handler.Follower.Browse, r.setViewer)
	socialgraph.GET("/:actor_id/leaders/", r.handler.Leader.Browse, r.setViewer)

	socialgraph.POST("/:target_id/followers/:source_id", r.handler.Follower.Add, r.authorize)
	socialgraph.DELETE("/:target_id/followers/:source_id", r.handler.Follower.Delete, r.authorize)

	socialgraph.GET("/:actor_id/mutuals/", r.handler.Mutual.Browse, r.authorize)

	socialgraph.GET("/:actor_id/followrequests/", r.handler.FollowRequest.Browse, r.authorize)
	socialgraph.PATCH("/:target_id/followrequests/:source_id", r.handler.FollowRequest.Edit, r.authorize)
	socialgraph.POST("/:target_id/followrequests/:source_id", r.handler.FollowRequest.Add, r.authorize)
	socialgraph.DELETE("/:target_id/followrequests/:source_id", r.handler.FollowRequest.Delete, r.authorize)

	socialgraph.GET("/:actor_id/blocks/", r.handler.Block.Browse, r.authorize)
	socialgraph.POST("/:target_id/blocks/:source_id", r.handler.Block.Add, r.authorize)
	socialgraph.DELETE("/:target_id/blocks/:source_id", r.handler.Block.Delete, r.authorize)
}
