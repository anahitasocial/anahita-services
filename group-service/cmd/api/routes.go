package main

import (
	"group/cmd/api/handlers"

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
	// groups — Browse and Read are public but still identify the caller
	// when a token is present, so the viewer drives access filtering.
	// Everything else requires a valid OAuth access token.
	groups := e.Group("groups")
	groups.GET("/", r.handler.Group.Browse, r.setViewer)
	groups.GET("/:id", r.handler.Group.Read, r.setViewer)
	groups.POST("/", r.handler.Group.Add, r.authorize)
	groups.PATCH("/:id", r.handler.Group.Edit, r.authorize)
	groups.DELETE("/:id", r.handler.Group.Delete, r.authorize)
	// What the deletion would cost, for the Danger zone. Same permission as
	// the deletion itself — see the handler.
	groups.GET("/:id/deletion-counts", r.handler.Group.DeletionCounts, r.authorize)
	// Hard delete. Super administrators only — enforced in the handler.
	groups.DELETE("/:id/purge", r.handler.Group.Purge, r.authorize)
	// Archive: permanent, destroys nothing. A group administrator may.
	groups.POST("/:id/archive", r.handler.Group.Archive, r.authorize)
	// Enable/disable: reversible, site administrators only.
	groups.PATCH("/:id/enabled", r.handler.Group.SetEnabled, r.authorize)
	// Undo, within the grace period. Groups cannot sign in, so this is their
	// equivalent of the person restore-at-login path.
	groups.POST("/:id/restore", r.handler.Group.Restore, r.authorize)

	// group edit access
	groups.PATCH("/:id/access", r.handler.Group.EditAccess, r.authorize)

	// group edit features
	groups.PATCH("/:id/features", r.handler.Group.EditFeatures, r.authorize)

	groups.GET("/:id/admins/", r.handler.Admin.Browse, r.authorize)
	groups.POST("/:id/admins/", r.handler.Admin.Add, r.authorize)
	groups.DELETE("/:id/admins/:admin_id", r.handler.Admin.Delete, r.authorize)
}
