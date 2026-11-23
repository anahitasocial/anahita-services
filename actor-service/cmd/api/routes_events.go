package main

import (
	"net/http"

	"actor/cmd/api/handlers"
	"actor/responses"

	"anahita-libs/apidoc"
	libs_responses "anahita-libs/responses"
)

// --- Events ---
// eventRoutes registers the routes at /events/: a profile like a group,
// with a time and a place. Everything a group has, an event has, from the
// same handler told a different kind; what only an event has is here as
// well. What is on, GET /events/, lists every event still to come that the
// viewer may see; an event is found as well from an invitation, from
// whoever hosts it, or by its address.
//
// The viewer's own events need a token (r.authorize). What is on, the
// featured ones, one event, who is going and its calendar file are public:
// no token is needed, and when one is sent the caller is known
// (r.setViewer).
func (r *Routes) eventRoutes(api *apidoc.Router, s *shared) {
	events := api.In("events").Group("events")

	// The viewer's own: what they answered, look after, or were invited
	// to. Before /:id, though a word is never mistaken for an id.
	events.GET("/mine", r.handler.Event.Mine, r.authorize).
		Summary("The viewer's events").
		Describe("The ones they said going or maybe to, look after, or were invited to.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventMineRequest{Limit: 20}).
		Sends(http.StatusOK, libs_responses.List[any]{})

	// A visitor gets the public ones.
	events.GET("/", r.handler.Event.Discover, r.setViewer).
		Summary("What is on").
		Describe("Every event still to come or on now that the viewer may see, whoever hosts it, "+
			"not called off, soonest first.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.EventDiscoverRequest{Limit: 20}).
		Sends(http.StatusOK, libs_responses.List[any]{})

	// The ones a super administrator features, for onboarding.
	events.GET("/featured", r.handler.Event.Featured, r.setViewer).
		Summary("The events the installation features").
		Describe("What onboarding offers a new member, most recently featured first. "+s.someOrLimited).
		Viewer(apidoc.Anyone).
		Takes(s.actorReq.Browse).
		Sends(http.StatusOK, libs_responses.List[any]{})

	events.GET("/:id", r.handler.Event.Read, r.setViewer).With(s.readGroup("event"))

	events.GET("/:id/attendees/", r.handler.Event.Attendees, r.setViewer).
		Summary("Who is going to an event, or may").
		Viewer(apidoc.Anyone).
		Takes(&handlers.EventAttendeesRequest{Limit: 20}).
		Sends(http.StatusOK, libs_responses.List[*libs_responses.ActorNodeItem]{}).
		Refuses(http.StatusForbidden, "the viewer may not see who is going").
		Refuses(http.StatusNotFound, "there is no such event")

	// For the "Add to calendar" button.
	events.GET("/:id/ics", r.handler.Event.ICS, r.setViewer).
		Summary("An event for a calendar").
		Describe("As iCalendar (RFC 5545), `text/calendar`, to add to one's own calendar.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.EventIDRequest{}).
		SendsNothing(http.StatusOK, "the event, as `text/calendar`").
		Refuses(http.StatusForbidden, "the viewer may not see it").
		Refuses(http.StatusNotFound, "there is no such event")

	// --- Making and looking after an event ---
	// Making and changing an event, deleting, purging, archiving, turning
	// it off, featuring and restoring it, its settings, its rules and its
	// admins, as a group's. Reading the rules is public: no token is needed,
	// and when one is sent the caller is known (r.setViewer). Every other
	// route needs a token (r.authorize), and the handlers check who may.
	events.POST("/", r.handler.Event.Add, r.authorize).
		Summary("Make an event").
		Describe("Its own, or hosted by a group whoever makes it administers.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventAddRequest{}).
		Sends(http.StatusCreated, responses.EventNode{}).
		Sends(http.StatusBadRequest, libs_responses.Refusal{}, "`invalid_timezone`, "+
			"`invalid_time`, `ends_before_start`, `too_long` or `already_over`; or not well "+
			"formed, without a body").
		Sends(http.StatusForbidden, libs_responses.Refusal{}, "`host_not_allowed`: the viewer "+
			"does not administer the group").
		Sends(http.StatusNotFound, libs_responses.Refusal{},
			"`host_not_allowed`: there is no such group")

	events.PATCH("/:id", r.handler.Event.Edit, r.authorize).
		Summary("Change an event").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventEditRequest{}).
		Sends(http.StatusOK, responses.EventNode{}).
		Sends(http.StatusBadRequest, libs_responses.Refusal{},
			"`invalid_timezone`, `invalid_time`, `ends_before_start` or `too_long`; or not well "+
				"formed, without a body").
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such event")

	events.DELETE("/:id", r.handler.Event.Delete, r.authorize).
		With(s.remove("event", s.actorReq.Delete))

	events.GET("/:id/deletion-counts", r.handler.Event.DeletionCounts, r.authorize).
		With(s.counts("event", handlers.GroupDeletionCounts{}))

	events.DELETE("/:id/purge", r.handler.Event.Purge, r.authorize).With(s.purge("event"))

	events.POST("/:id/archive", r.handler.Event.Archive, r.authorize).With(s.archive("event"))

	events.PATCH("/:id/enabled", r.handler.Event.SetEnabled, r.authorize).With(s.enabled)

	events.PATCH("/:id/featured", r.handler.Event.SetFeatured, r.authorize).With(s.featured)

	events.POST("/:id/restore", r.handler.Event.Restore, r.authorize).With(s.restore("event"))

	events.PATCH("/:id/access", r.handler.Event.EditAccess, r.authorize).
		With(s.access(s.actorReq.EditAccess, responses.EventNode{}))

	events.PATCH("/:id/graph-access", r.handler.Event.EditGraphAccess, r.authorize).
		With(s.graphAccess(responses.EventNode{}))

	events.PATCH("/:id/search-engines", r.handler.Event.EditSearchEngines, r.authorize).
		With(s.searchEngines(responses.EventNode{}))

	events.GET("/:id/rules", r.handler.Event.ReadRules, r.setViewer).With(s.readRules("event"))

	events.PUT("/:id/rules", r.handler.Event.SetRules, r.authorize).With(s.setRules("event"))

	events.POST("/:id/rules/agree", r.handler.Event.AgreeRules, r.authorize).
		With(s.agreeRules("event"))

	events.PATCH("/:id/features", r.handler.Event.EditFeatures, r.authorize).
		With(s.features(s.actorReq.EditFeatures, responses.EventNode{}))

	events.GET("/:id/admins/", r.handler.EventAdmin.Browse, r.authorize).With(s.eventAdmins)

	events.POST("/:id/admins/", r.handler.EventAdmin.Add, r.authorize).With(s.addEventAdmin)

	events.DELETE("/:id/admins/:admin_id", r.handler.EventAdmin.Delete, r.authorize).
		With(s.removeEventAdmin)

	// --- Answering an event, and calling it off ---
	// Every route needs a token (r.authorize).
	events.PUT("/:id/rsvp", r.handler.Event.Rsvp, r.authorize).
		Summary("Say going, or maybe").
		Describe("Answers with the event.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventRsvpRequest{}).
		Sends(http.StatusOK, responses.EventNode{}).
		Refuses(http.StatusForbidden, "the viewer may not go to it").
		Refuses(http.StatusNotFound, "there is no such event").
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`event_full`: every place is taken; "+
			"maybe is still open; or `rules_not_agreed`: it is a group or an event with rules the "+
			"viewer has not agreed to; tick each and agree (`POST "+
			"/groups|events/:id/rules/agree`), then ask again").
		Sends(http.StatusGone, libs_responses.Refusal{}, "`event_closed`: it is over or called off")

	events.DELETE("/:id/rsvp", r.handler.Event.Unrsvp, r.authorize).
		Summary("Not going after all").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventIDRequest{}).
		SendsNothing(http.StatusNoContent, "done").
		Refuses(http.StatusForbidden, "the viewer may not").
		Refuses(http.StatusNotFound, "there is no such event")

	events.POST("/:id/cancel", r.handler.Event.Cancel, r.authorize).
		Summary("Call an event off").
		Describe("It is kept, saying so. Answers with it.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventIDRequest{}).
		Sends(http.StatusOK, responses.EventNode{}).
		Refuses(http.StatusForbidden, "the viewer does not look after it").
		Refuses(http.StatusNotFound, "there is no such event")
}
