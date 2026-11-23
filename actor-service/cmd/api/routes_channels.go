package main

import (
	"net/http"

	"actor/cmd/api/handlers"
	"actor/responses"

	"anahita-libs/apidoc"
	libs_responses "anahita-libs/responses"
)

// --- Channels ---
// channelRoutes registers the routes at /channels/: a profile only it posts
// on, written by its admins (roadmap row 147), read and looked after as a
// group is. It is made here, or by syndication-service from a website's
// feed once its domain is proved (POST /channels/_connect). Listing
// channels and reading one are public: no token is needed, and when one is
// sent the caller is known (r.setViewer). Making and changing one need a
// token (r.authorize).
func (r *Routes) channelRoutes(api *apidoc.Router, s *shared) {
	channels := api.In("channels").Group("channels")

	channels.GET("/", r.handler.Channel.Browse, r.setViewer).
		Summary("Channels").
		Describe("Those the viewer may see. "+s.someOrLimited).
		Viewer(apidoc.Anyone).
		Takes(s.actorReq.Browse).
		Sends(http.StatusOK, libs_responses.List[any]{}).
		Refuses(http.StatusForbidden, "the viewer may not list channels")

	channels.GET("/:id", r.handler.Channel.Read, r.setViewer).With(s.readGroup("channel"))

	// Made as a group is, with or without a feed (roadmap row 147, phase
	// 2), by whoever CHANNELS_FROM allows.
	channels.POST("/", r.handler.Channel.Add, r.authorize).
		Summary("Make a channel").
		Describe("A profile only it posts on, written by its admins; whoever makes it is its "+
			"first admin. Left without `access`, only its admins can see it until they open it. A "+
			"website's feed is connected to it apart (POST /channels/_connect).").
		Viewer(apidoc.SignedIn).
		Takes(s.actorReq.Add).
		Sends(http.StatusCreated, responses.ChannelNode{}).
		Sends(http.StatusUnprocessableEntity, libs_responses.AudienceRefusal{},
			"`access_not_allowed`: public, where nothing new may be, with the levels it may have")

	channels.PATCH("/:id", r.handler.Channel.Edit, r.authorize).
		Summary("Change a channel").
		Describe("Its name and its description. Its handle and its site stay as they were "+
			"connected.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ChannelEditRequest{}).
		Sends(http.StatusOK, responses.ChannelNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such channel")

	// --- Deleting a channel ---
	// Every route needs a token (r.authorize); the handlers check who may.
	channels.DELETE("/:id", r.handler.Channel.Delete, r.authorize).
		With(s.remove("channel", s.actorReq.Delete))

	channels.GET("/:id/deletion-counts", r.handler.Channel.DeletionCounts, r.authorize).
		With(s.counts("channel", handlers.GroupDeletionCounts{}))

	channels.DELETE("/:id/purge", r.handler.Channel.Purge, r.authorize).With(s.purge("channel"))

	// --- A channel's events ---
	// The events a channel hosts (roadmap row 147, phase 5). Public: no
	// token is needed, and when one is sent the caller is known
	// (r.setViewer).
	channels.GET("/:id/events/", r.handler.Event.HostedByChannel, r.setViewer).
		Summary("The events a channel hosts").
		Describe("Those the viewer may see. Each an event, or what is shown of one the viewer may "+
			"only answer as a follower of the channel.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.EventHostedRequest{Limit: 20}).
		Sends(http.StatusOK, libs_responses.List[any]{}).
		Refuses(http.StatusForbidden, "the viewer may not see the channel").
		Refuses(http.StatusNotFound, "there is no such channel")

	// --- Looking after a channel ---
	// Turning it off, featuring it, restoring it, and what its new photos
	// and videos start with; then, as a group's (roadmap row 147, phase 2),
	// who sees it and may ask to follow it, who sees its followers, what it
	// offers, archiving, spam taken away by an administrator, and its
	// admins. Every route needs a token (r.authorize).
	channels.PATCH("/:id/enabled", r.handler.Channel.SetEnabled, r.authorize).With(s.enabled)

	channels.PATCH("/:id/featured", r.handler.Channel.SetFeatured, r.authorize).With(s.featured)

	channels.POST("/:id/restore", r.handler.Channel.Restore, r.authorize).With(s.restore("channel"))

	channels.PATCH("/:id/download-default", r.handler.Channel.EditDownloadDefault, r.authorize).
		With(s.downloads(responses.ChannelNode{}))

	channels.PATCH("/:id/access", r.handler.Channel.EditAccess, r.authorize).
		With(s.access(s.actorReq.EditAccess, responses.ChannelNode{}))

	channels.PATCH("/:id/graph-access", r.handler.Channel.EditGraphAccess, r.authorize).
		With(s.graphAccess(responses.ChannelNode{}))

	channels.PATCH("/:id/search-engines", r.handler.Channel.EditSearchEngines, r.authorize).
		With(s.searchEngines(responses.ChannelNode{}))

	channels.PATCH("/:id/features", r.handler.Channel.EditFeatures, r.authorize).
		With(s.features(s.actorReq.EditFeatures, responses.ChannelNode{}))

	channels.POST("/:id/archive", r.handler.Channel.Archive, r.authorize).With(s.archive("channel"))

	channels.POST("/:id/spam", r.handler.Channel.DeleteAsSpam, r.authorize).With(s.spam("channel"))

	channels.GET("/:id/admins/", r.handler.ChannelAdmin.Browse, r.authorize).With(s.channelAdmins)

	channels.POST("/:id/admins/", r.handler.ChannelAdmin.Add, r.authorize).With(s.addChannelAdmin)

	channels.DELETE("/:id/admins/:admin_id", r.handler.ChannelAdmin.Delete, r.authorize).
		With(s.removeChannelAdmin)
}
