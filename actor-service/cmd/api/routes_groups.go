package main

import (
	"net/http"

	"actor/cmd/api/handlers"
	"actor/responses"

	"anahita-libs/apidoc"
	libs_responses "anahita-libs/responses"
)

// --- Groups ---
// groupRoutes registers the routes at /groups/: made by somebody signed in,
// looked after by its admins, and followed by people. Listing groups and
// reading one are public: no token is needed, and when one is sent the
// caller is known (r.setViewer). Every other route of this first part
// needs a token (r.authorize).
func (r *Routes) groupRoutes(api *apidoc.Router, s *shared) {
	groups := api.In("groups").Group("groups")

	groups.GET("/", r.handler.Group.Browse, r.setViewer).
		Summary("Groups").
		Describe("Those the viewer may see. "+s.someOrLimited).
		Viewer(apidoc.Anyone).
		Takes(s.actorReq.Browse).
		Sends(http.StatusOK, libs_responses.List[any]{}).
		Refuses(http.StatusForbidden, "the viewer may not list groups")

	groups.GET("/:id", r.handler.Group.Read, r.setViewer).With(s.readGroup("group"))

	groups.POST("/", r.handler.Group.Add, r.authorize).
		Summary("Make a group").
		Describe("Whoever makes it is its first administrator. Left without `access`, only its "+
			"administrators can see it until they open it.").
		Viewer(apidoc.SignedIn).
		Takes(s.actorReq.Add).
		Sends(http.StatusCreated, responses.GroupNode{}).
		Sends(http.StatusUnprocessableEntity, libs_responses.AudienceRefusal{},
			"`access_not_allowed`: public, where nothing new may be, with the levels it may have")

	groups.PATCH("/:id", r.handler.Group.Edit, r.authorize).
		Summary("Change a group").
		Viewer(apidoc.SignedIn).
		Takes(s.actorReq.Edit).
		Sends(http.StatusOK, responses.GroupNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such group")

	// --- Deleting, archiving and restoring a group ---
	// Every route needs a token (r.authorize). What deleting would take with
	// it, for the Danger zone, has the same permission as deleting: see the
	// handler. Purging is for super administrators, removing as spam for
	// administrators, archiving for the group's admins, turning it off for
	// site administrators, and featuring it for super administrators; the
	// handlers check. Restoring is for within the thirty days: groups
	// cannot sign in, so this is their equivalent of a person being
	// restored when they sign in.
	groups.DELETE("/:id", r.handler.Group.Delete, r.authorize).
		With(s.remove("group", s.actorReq.Delete))

	groups.GET("/:id/deletion-counts", r.handler.Group.DeletionCounts, r.authorize).
		With(s.counts("group", handlers.GroupDeletionCounts{}))

	groups.DELETE("/:id/purge", r.handler.Group.Purge, r.authorize).With(s.purge("group"))

	groups.POST("/:id/spam", r.handler.Group.DeleteAsSpam, r.authorize).With(s.spam("group"))

	groups.POST("/purge-batch", r.handler.Group.PurgeBatch, r.authorize).With(s.purgeBatch("group"))

	groups.GET("/purge-batch/:batch_id", r.handler.Group.PurgeProgress, r.authorize).
		With(s.purgeProgress)

	groups.POST("/:id/archive", r.handler.Group.Archive, r.authorize).With(s.archive("group"))

	groups.PATCH("/:id/enabled", r.handler.Group.SetEnabled, r.authorize).With(s.enabled)

	groups.PATCH("/:id/featured", r.handler.Group.SetFeatured, r.authorize).With(s.featured)

	groups.POST("/:id/restore", r.handler.Group.Restore, r.authorize).With(s.restore("group"))

	// --- A group's settings and rules ---
	// Who may see it and its followers, what its new photos and videos
	// start with, what may be posted on it, and its rules. Reading the rules
	// is public: no token is needed, and when one is sent the caller is
	// known (r.setViewer). Every other route needs a token (r.authorize).
	groups.PATCH("/:id/access", r.handler.Group.EditAccess, r.authorize).
		With(s.access(s.actorReq.EditAccess, responses.GroupNode{}))

	groups.PATCH("/:id/graph-access", r.handler.Group.EditGraphAccess, r.authorize).
		With(s.graphAccess(responses.GroupNode{}))

	groups.PATCH("/:id/search-engines", r.handler.Group.EditSearchEngines, r.authorize).
		With(s.searchEngines(responses.GroupNode{}))

	groups.PATCH("/:id/download-default", r.handler.Group.EditDownloadDefault, r.authorize).
		With(s.downloads(responses.GroupNode{}))

	groups.PATCH("/:id/features", r.handler.Group.EditFeatures, r.authorize).
		With(s.features(s.actorReq.EditFeatures, responses.GroupNode{}))

	groups.GET("/:id/rules", r.handler.Group.ReadRules, r.setViewer).With(s.readRules("group"))

	groups.PUT("/:id/rules", r.handler.Group.SetRules, r.authorize).With(s.setRules("group"))

	groups.POST("/:id/rules/agree", r.handler.Group.AgreeRules, r.authorize).
		With(s.agreeRules("group"))

	// --- A group's events and admins ---
	// The events it hosts are public: no token is needed, and when one is
	// sent the caller is known (r.setViewer). Its admins need a token
	// (r.authorize).
	groups.GET("/:id/events/", r.handler.Event.HostedBy, r.setViewer).
		Summary("The events a group hosts").
		Describe("Those the viewer may see. Each an event, or what is shown of one the viewer may "+
			"only answer as a follower of the group.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.EventHostedRequest{Limit: 20}).
		Sends(http.StatusOK, libs_responses.List[any]{}).
		Refuses(http.StatusForbidden, "the viewer may not see the group").
		Refuses(http.StatusNotFound, "there is no such group")

	groups.GET("/:id/admins/", r.handler.Admin.Browse, r.authorize).With(s.groupAdmins)

	groups.POST("/:id/admins/", r.handler.Admin.Add, r.authorize).With(s.addGroupAdmin)

	groups.DELETE("/:id/admins/:admin_id", r.handler.Admin.Delete, r.authorize).
		With(s.removeGroupAdmin)
}
