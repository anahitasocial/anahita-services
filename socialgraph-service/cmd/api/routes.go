package main

import (
	an_mw "anahita-libs/middleware"
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
	// Who follows whom is not part of a preview: a visitor is shown the
	// counts on a profile, and has to sign in for the names.
	socialgraph.GET("/:actor_id/followers/", r.handler.Follower.Browse, r.setViewer, an_mw.MembersInPreview())
	socialgraph.GET("/:actor_id/leaders/", r.handler.Leader.Browse, r.setViewer, an_mw.MembersInPreview())

	socialgraph.POST("/:target_id/followers/:source_id", r.handler.Follower.Add, r.authorize)
	socialgraph.DELETE("/:target_id/followers/:source_id", r.handler.Follower.Delete, r.authorize)

	socialgraph.GET("/:actor_id/mutuals/", r.handler.Mutual.Browse, r.authorize)

	socialgraph.GET("/:actor_id/follow-requests/", r.handler.FollowRequest.Browse, r.authorize)
	socialgraph.PATCH("/:target_id/follow-requests/:source_id", r.handler.FollowRequest.Edit, r.authorize)
	socialgraph.POST("/:target_id/follow-requests/:source_id", r.handler.FollowRequest.Add, r.authorize)
	socialgraph.DELETE("/:target_id/follow-requests/:source_id", r.handler.FollowRequest.Delete, r.authorize)

	socialgraph.GET("/:actor_id/blocks/", r.handler.Block.Browse, r.authorize)
	socialgraph.POST("/:target_id/blocks/:source_id", r.handler.Block.Add, r.authorize)
	socialgraph.DELETE("/:target_id/blocks/:source_id", r.handler.Block.Delete, r.authorize)
}
