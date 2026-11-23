package main

import (
	"net/http"

	"activitypub/cmd/api/handlers"

	"anahita-libs/apidoc"
	"anahita-libs/responses"

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
// the descriptions: docs/api/activitypub-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("activitypub-service", "The fediverse",
		"This installation on the fediverse, by ActivityPub: what other servers ask of it "+
			"and send to it, a person's choice to be on it, finding somebody elsewhere, and the "+
			"servers it refuses (federation.md). Optional: an installation may not run it.")
	api := doc.On(e)

	// What other servers ask of this installation, and send to it.
	r.serverRoutes(api)

	// Where somebody from another server, on a page here and not signed in,
	// is sent to act on it from their own server (roadmap row 134). Outside
	// the group below: it needs no token.
	api.GET("/activitypub/intent", r.handler.Intent, r.setViewer).
		Summary("Where somebody from another server acts on something here").
		Describe("Given their handle, the page on their own server where they follow a profile "+
			"here, or answer, like or repost a post here: their server's FEP-3b86 intent, else its "+
			"OStatus subscribe page. With none, `error` is `intent_none` and `address` is what to "+
			"search for there. Thirty in ten minutes from one address.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.IntentRequest{}).
		Unbound().
		Sends(http.StatusOK, handlers.Intent{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`intent_handle`: not a handle on another server").
		Sends(http.StatusNotFound, responses.Refusal{}, "`intent_not_out`: the profile or the post is not on the fediverse").
		Sends(http.StatusConflict, responses.Refusal{}, "the installation is not on the fediverse: "+
			"`site_private` or `site_unreachable`").
		Sends(http.StatusTooManyRequests, responses.Refusal{}, "`too_many_intents`")

	// --- Asked by people here ---
	// Everything under /activitypub is asked by somebody signed in here, so
	// every route needs a token (r.authorize). Who may do what beyond that
	// is checked in the handlers.
	activitypub := api.Group("activitypub", r.authorize)

	// A person's own choice to be on the fediverse, and moving their
	// account out and in.
	r.accountRoutes(activitypub)

	// Finding somebody on another server, and reading them or their
	// posts' pictures again.
	r.remoteRoutes(activitypub)

	// The servers and the accounts this installation refuses.
	r.refusalRoutes(activitypub)

	return doc
}
