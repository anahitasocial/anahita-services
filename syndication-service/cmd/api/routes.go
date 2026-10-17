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
// Only connecting a website is here. The website, once made, is a profile
// served by actor-service under /websites, as a group is under /groups.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("syndication-service", "Websites",
		"Connecting a website: its feed found, its domain proved, and a Website made of it, a profile whose posts are the feed's items (websites.md). Who may is WEBSITES_FROM, administrators unless the installation says otherwise.")
	api := doc.On(e)
	connect := api.Group("websites/_connect")

	notAllowed := func(route *apidoc.Route) *apidoc.Route {
		return route.
			Refuses(http.StatusUnauthorized, "the viewer is not signed in").
			Refuses(http.StatusForbidden, "the viewer may not connect a website: see WEBSITES_FROM")
	}

	connect.POST("", r.handler.Connect.Start, r.authorize).With(func(route *apidoc.Route) {
		notAllowed(route.
			Summary("Begin connecting a website").
			Describe("The site's feed is found: the address itself, or the feed the page names with `<link rel=\"alternate\">`. It must be on the site's domain. The answer is the token, to be put in a TXT record or a file, with a name and a handle to begin with.").
			Viewer(apidoc.SignedIn).
			Takes(&handlers.ConnectStartRequest{}).
			Sends(http.StatusCreated, handlers.Connection{}).
			Refuses(http.StatusBadRequest, "`not_a_domain`, `no_feed`, `unreadable` or `feed_not_on_domain`, as `error`").
			Refuses(http.StatusConflict, "`feed_taken`: the feed already has a Website").
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

	connect.POST("/:id", r.handler.Connect.Make, r.authorize).With(func(route *apidoc.Route) {
		notAllowed(route.
			Summary("Make the Website").
			Describe("Once its domain is proved. Whoever proved it is its first administrator and its first follower; its feed's ten newest items are posted, the rest marked seen, and what is published afterwards is posted as it comes.").
			Viewer(apidoc.SignedIn).
			Takes(&handlers.ConnectMakeRequest{}).
			Sends(http.StatusCreated, handlers.Made{}).
			Refuses(http.StatusBadRequest, "`no_feed`, `unreadable` or `feed_not_on_domain`, as `error`").
			Refuses(http.StatusNotFound, "there is no such connection begun by the viewer").
			Refuses(http.StatusConflict, "`not_proved`, `feed_taken` or `handle_taken`, as `error`"))
	})

	// Where a WebSub hub calls back: not signed in, since a hub has no
	// account here; the address's signature says which website it is about.
	// Through SetViewer, so that a members-only site, which does not
	// federate, refuses it, and its feeds are read on the clock.
	hub := api.Group("websites/_websub")
	hub.GET("/:id/:signature", r.handler.WebSub.Verify, r.setViewer).
		Summary("A hub checks a subscription").
		Describe("WebSub: the hub a Website's feed names asks whether its subscription was asked for here, and is answered with its challenge.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.WebSubRequest{}).
		Unbound().
		Sends(http.StatusOK, "", "the challenge, as plain text").
		Refuses(http.StatusBadRequest, "it is not a check this answers").
		Refuses(http.StatusNotFound, "there is no such Website, or no subscription was asked for")
	hub.POST("/:id/:signature", r.handler.WebSub.Notify, r.setViewer).
		Summary("A hub says a feed changed").
		Describe("The Website's feed is read from its site at once, at most once a minute; what the hub sends is not read.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.WebSubRequest{}).
		Unbound().
		SendsNothing(http.StatusAccepted, "it will be read").
		Refuses(http.StatusNotFound, "there is no such Website, or no subscription was asked for")

	return doc
}
