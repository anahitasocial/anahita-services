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

	// Where other servers send what somebody did: one for the whole
	// installation, and one for each person. They are the same door.
	e.POST("/inbox", r.handler.Inbox, r.setViewer)

	ap := e.Group("ap")
	ap.POST("/users/:uuid/inbox", r.handler.Inbox, r.setViewer)
	ap.GET("/actor", r.handler.InstanceActor, r.setViewer)
	ap.GET("/users/:uuid", r.handler.Actor, r.setViewer)
	ap.GET("/users/:uuid/followers", r.handler.Followers, r.setViewer)
	ap.GET("/users/:uuid/following", r.handler.Following, r.setViewer)
	ap.GET("/users/:uuid/outbox", r.handler.Outbox, r.setViewer)
	ap.GET("/notes/:uuid", r.handler.Note, r.setViewer)
	// One of a photo post's pictures, for a server that was sent the post.
	ap.GET("/notes/:uuid/pictures/:position", r.handler.Picture, r.setViewer)

	// A person's own choice to be on the fediverse.
	activitypub := e.Group("activitypub")
	activitypub.Use(r.authorize)
	activitypub.GET("/settings", r.handler.ReadSettings)
	activitypub.PUT("/settings", r.handler.EditSettings)
	// Finding somebody on another server.
	activitypub.GET("/lookup", r.handler.Lookup)
	// The servers this installation refuses. Super administrators only:
	// the handlers check.
	activitypub.GET("/blocks", r.handler.BrowseBlocks)
	activitypub.POST("/blocks", r.handler.AddBlock)
	activitypub.DELETE("/blocks/:domain", r.handler.DeleteBlock)
}
