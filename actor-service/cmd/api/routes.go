package main

import (
	"actor/cmd/api/handlers"

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
	// Each kind of actor keeps its own address: /people/ and /groups/. The
	// gateway sends both here.

	// people — Browse and Read are public but still identify the caller
	// when a token is present, so the viewer drives access filtering.
	// Everything else requires a valid OAuth access token.
	people := e.Group("people")
	people.GET("/", r.handler.Person.Browse, r.setViewer)
	people.GET("/:username", r.handler.Person.Read, r.setViewer)
	people.PATCH("/:id", r.handler.Person.Edit, r.authorize)
	// :id, not :username. The client sends node.id here (api/create.js
	// deleteItem builds /people/{id}), so this route matched the username
	// against a number and answered 404 to every delete — the flow could never
	// have run. PATCH above already keys on the id, and a group's delete keys
	// on the id too, so this is the consistent shape as well as the working
	// one.
	people.DELETE("/:id", r.handler.Person.Delete, r.authorize)
	// What the deletion would cost, for the Danger zone. Same permission as
	// the deletion itself — see the handler.
	people.GET("/:id/deletion-counts", r.handler.Person.DeletionCounts, r.authorize)
	// Hard delete. A separate route from DELETE /people/:id because the
	// difference between recoverable and gone should not be a parameter.
	// Super administrators only — enforced in the handler.
	people.DELETE("/:id/purge", r.handler.Person.Purge, r.authorize)
	// The same for many at once, and how far a request has got.
	people.POST("/purge-batch", r.handler.Person.PurgeBatch, r.authorize)
	people.GET("/purge-batch/:batch_id", r.handler.Person.PurgeProgress, r.authorize)
	// Archive: permanent, destroys nothing. Self or a super administrator.
	people.POST("/:id/archive", r.handler.Person.Archive, r.authorize)
	// Enable/disable: reversible, administrators only.
	people.PATCH("/:id/enabled", r.handler.Person.SetEnabled, r.authorize)
	// Featuring: what onboarding offers to follow. Super administrators only.
	people.PATCH("/:id/featured", r.handler.Person.SetFeatured, r.authorize)

	// Edit access
	people.PATCH("/:id/access", r.handler.Person.EditAccess, r.authorize)
	people.PATCH("/:id/reposts-on-profile", r.handler.Person.EditRepostsOnProfile, r.authorize)

	// Edit features
	people.PATCH("/:id/features", r.handler.Person.EditFeatures, r.authorize)

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
	groups.POST("/purge-batch", r.handler.Group.PurgeBatch, r.authorize)
	groups.GET("/purge-batch/:batch_id", r.handler.Group.PurgeProgress, r.authorize)
	// Archive: permanent, destroys nothing. A group administrator may.
	groups.POST("/:id/archive", r.handler.Group.Archive, r.authorize)
	// Enable/disable: reversible, site administrators only.
	groups.PATCH("/:id/enabled", r.handler.Group.SetEnabled, r.authorize)
	// Featuring: what onboarding offers to follow. Super administrators only.
	groups.PATCH("/:id/featured", r.handler.Group.SetFeatured, r.authorize)
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
