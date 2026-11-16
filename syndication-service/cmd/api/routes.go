package main

import (
	"net/http"

	"syndication/cmd/api/handlers"

	"anahita-libs/apidoc"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
	setViewer echo.MiddlewareFunc
}

func NewRoutes(handler *handlers.Handlers, authorize, setViewer echo.MiddlewareFunc) *Routes {
	return &Routes{handler: handler, authorize: authorize, setViewer: setViewer}
}

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/syndication-service.json is made of them.
//
// Only a channel's feed is here. The channel, once made, is a profile
// served by actor-service under /channels, as a group is under /groups.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("syndication-service", "Channels",
		"A channel's feed: a website's feed found, its domain proved, and the feed connected to a channel its admins look after, whose notes are then the feed's items (channels.md). Who may connect one is CHANNELS_FROM, administrators unless the installation says otherwise.")
	api := doc.On(e)
	connect := api.Group("channels/_connect")

	notAllowed := func(route *apidoc.Route) *apidoc.Route {
		return route.
			Refuses(http.StatusUnauthorized, "the viewer is not signed in").
			Refuses(http.StatusForbidden, "the viewer may not connect a channel: see CHANNELS_FROM")
	}

	connect.POST("", r.handler.Connect.Start, r.authorize).With(func(route *apidoc.Route) {
		notAllowed(route.
			Summary("Begin connecting a channel").
			Describe("The site's feed is found: the address itself, or the feed the page names with `<link rel=\"alternate\">`. It must be on the site's domain. The answer is the token, to be put in a TXT record or a file.").
			Viewer(apidoc.SignedIn).
			Takes(&handlers.ConnectStartRequest{}).
			Sends(http.StatusCreated, handlers.Connection{}).
			Refuses(http.StatusBadRequest, "`not_a_domain`, `no_feed`, `unreadable`, `feed_not_on_domain` or `private_feed` (behind a login, or a secret in its address: a channel is public), as `error`").
			Refuses(http.StatusConflict, "`feed_taken`: the feed already has a Channel").
			Refuses(http.StatusTooManyRequests, "the viewer has begun too many lately"))
	})

	connect.POST("/:id/check", r.handler.Connect.Check, r.authorize).With(func(route *apidoc.Route) {
		notAllowed(route.
			Summary("Look for the token").
			Describe("In the TXT record at `_anahita.<domain>`, then in `https://<host>/.well-known/anahita-verification.txt`. Asked as often as the one connecting likes.").
			Viewer(apidoc.SignedIn).
			Takes(&handlers.ConnectIDRequest{}).
			Sends(http.StatusOK, handlers.Checked{}).
			Refuses(http.StatusNotFound, "there is no such connection begun by the viewer"))
	})

	// A channel's feed, in its settings (roadmap row 147, phase 4). Under
	// _connect, where the gateway sends syndication-service's addresses.
	feed := connect.Group("/feeds")
	feed.GET("/:channel_id", r.handler.Connect.ShowFeed, r.authorize).
		Summary("A channel's feed").
		Describe("The feed a channel reads, for its admins and the site's administrators.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ChannelFeedRequest{}).
		Sends(http.StatusOK, handlers.ChannelFeed{}).
		Refuses(http.StatusUnauthorized, "the viewer is not signed in").
		Refuses(http.StatusForbidden, "the viewer is not one of the channel's admins").
		Refuses(http.StatusNotFound, "there is no such channel, or it has no feed")

	feed.PUT("/:channel_id", r.handler.Connect.ConnectFeed, r.authorize).With(func(route *apidoc.Route) {
		notAllowed(route.
			Summary("Connect a channel's feed").
			Describe("Once the site's domain is proved (the two calls above). The channel's admins connect it; whoever proved the domain must be the one connecting. Its feed's ten newest items are posted, the rest marked seen, and what is published afterwards is posted as it comes. Where the channel names no website, the proved site becomes its website.").
			Viewer(apidoc.SignedIn).
			Takes(&handlers.ChannelFeedConnectRequest{}).
			Sends(http.StatusCreated, handlers.ChannelFeed{}).
			Refuses(http.StatusBadRequest, "`no_feed`, `unreadable`, `feed_not_on_domain` or `private_feed`, as `error`").
			Refuses(http.StatusNotFound, "there is no such channel, or no such connection begun by the viewer").
			Refuses(http.StatusConflict, "`not_proved`, `feed_taken` or `has_feed` (one feed a channel), as `error`"))
	})

	feed.DELETE("/:channel_id", r.handler.Connect.DisconnectFeed, r.authorize).
		Summary("Disconnect a channel's feed").
		Describe("The feed is read no more. The notes it made stay, as their site wrote them; connecting the same feed again posts none of them twice.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ChannelFeedRequest{}).
		SendsNothing(http.StatusNoContent, "it is disconnected").
		Refuses(http.StatusUnauthorized, "the viewer is not signed in").
		Refuses(http.StatusForbidden, "the viewer is not one of the channel's admins").
		Refuses(http.StatusNotFound, "there is no such channel, or it has no feed")

	// Where a WebSub hub calls back: not signed in, since a hub has no
	// account here; the address's signature says which channel it is about.
	// Through SetViewer, so that a members-only site, which does not
	// federate, refuses it, and its feeds are read on the clock.
	hub := api.Group("channels/_websub")
	hub.GET("/:id/:signature", r.handler.WebSub.Verify, r.setViewer).
		Summary("A hub checks a subscription").
		Describe("WebSub: the hub a Channel's feed names asks whether its subscription was asked for here, and is answered with its challenge.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.WebSubRequest{}).
		Unbound().
		Sends(http.StatusOK, "", "the challenge, as plain text").
		Refuses(http.StatusBadRequest, "it is not a check this answers").
		Refuses(http.StatusNotFound, "there is no such Channel, or no subscription was asked for")
	hub.POST("/:id/:signature", r.handler.WebSub.Notify, r.setViewer).
		Summary("A hub says a feed changed").
		Describe("The Channel's feed is read from its site at once, at most once a minute; what the hub sends is not read.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.WebSubRequest{}).
		Unbound().
		SendsNothing(http.StatusAccepted, "it will be read").
		Refuses(http.StatusNotFound, "there is no such Channel, or no subscription was asked for")

	return doc
}
