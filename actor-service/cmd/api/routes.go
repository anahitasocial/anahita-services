package main

import (
	"net/http"

	"actor/cmd/api/handlers"

	"anahita-libs/apidoc"

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

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/actor-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("actor-service", "Profiles",
		"People, groups, events and channels: reading and changing them, who may see them and "+
			"what may be posted on them, their admins, and deleting, archiving and purging them "+
			"(account-lifecycle.md).").
		// The reference shows each kind apart, and what every kind has in a
		// section of its own: the routes each kind has at its own address,
		// alike, are marked Shared, and listed under each kind after its own.
		Section("profiles", "Profiles",
			"What every profile has, whatever its kind (profiles.md): who may see it and what may "+
				"be posted on it, its admins, being turned off, archived, deleted and restored, being "+
				"featured, and what its photos and videos start with about being downloaded. Each "+
				"kind has these at its own address, `/people/`, `/groups/`, `/events/` or "+
				"`/channels/`, and they are listed under each kind as every profile's. Here as well: "+
				"which profile an alias is, of any kind.").
		Section("people", "People",
			"Somebody with an account, at their username (people.md): their own posts and "+
				"replies, who may quote them, the photos and videos they are tagged in, and what they "+
				"show of their reposts. A person is made by signing up (auth-service), not here.").
		Section("groups", "Groups",
			"Made by somebody signed in, looked after by its admins, and followed by people "+
				"(groups.md): its rules, agreed to before following it, and the events it hosts.").
		Section("events", "Events",
			"A group with a time and a place (events.md): going or maybe, who is going, a file "+
				"for a calendar, calling it off, its rules, and what is on for everybody.").
		Section("channels", "Channels",
			"A channel's feed as a profile (channels.md), followed here and from the fediverse, "+
				"and looked after by its admins. It is made by connecting the site, which proves its "+
				"domain (syndication-service, `POST /channels/_connect`), so it has no route here to "+
				"make one.")
	api := doc.On(e)

	// Each kind of profile keeps its own address, /people/, /groups/,
	// /channels/ and /events/, and the gateway sends each here. What every
	// kind has alike is described once, in s, and registered under each
	// kind in its own file. The order below is the order of the reference.
	s := newShared()

	// Which profile an alias is, of any kind (below).
	r.profileRoutes(api)

	// People, their settings and their tags (routes_people.go).
	r.peopleRoutes(api, s)

	// Groups, their rules, events and admins (routes_groups.go).
	r.groupRoutes(api, s)

	// Channels, made here or from a website's feed (routes_channels.go).
	r.channelRoutes(api, s)

	// Events, what is on, and answering them (routes_events.go).
	r.eventRoutes(api, s)

	return doc
}

// --- Aliases ---
// Which profile an alias is, of any kind: see handlers/handle.go. Public:
// no token is needed, and when one is sent the caller is known
// (r.setViewer).
func (r *Routes) profileRoutes(api *apidoc.Router) {
	api.In("profiles").GET("/actors/:alias", r.handler.Person.Handle, r.setViewer).
		Summary("What profile an alias is").
		Describe("Of any kind: what its own address begins with, and its id. The web app finds "+
			"`/@ana` by it.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.HandleRequest{}).
		Unbound().
		Sends(http.StatusOK, handlers.HandleResponse{}).
		Refuses(http.StatusNotFound, "there is no such profile that the viewer may see")
}
