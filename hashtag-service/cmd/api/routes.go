package main

import (
	"net/http"

	"hashtag/cmd/api/handlers"
	"hashtag/requests"
	"hashtag/responses"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	libs_requests "anahita-libs/requests"
	libs_responses "anahita-libs/responses"

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
// the descriptions: docs/api/hashtag-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("hashtag-service", "Hashtags",
		"The hashtags written in posts, and the posts each one is in.")
	api := doc.On(e)

	// hashtags — the read routes are public but still identify the
	// caller when a token is present, so the viewer drives access
	// filtering.
	hashtags := api.Group("hashtags")
	hashtags.GET("/_browse/trending", r.handler.Hashtag.BrowseTrending, r.setViewer).
		Summary("Hashtags used most lately").
		Describe("The hashtags the most profiles used in a window of time that ends now: the writers of public posts carrying it, by when they were written, and the public profiles (people, groups, events, websites) tagged with it, by when they were tagged; each profile once, whatever its kind, here or elsewhere; `profiles` says how many. `time_window` left out, the installation's own (`TRENDING_WINDOW`; daily where it federates, monthly where not), widened while it holds fewer than ten; `window` says which. One page, all there is: up to `limit`.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewHashtagRequest().BrowseTrending).
		Sends(http.StatusOK, libs_responses.Trending[*responses.HashtagNodeItem]{})
	hashtags.GET("/_browse/following", r.handler.Hashtag.Following, r.authorize).
		Summary("The hashtags the viewer follows").
		Describe("Those in the viewer's custom feeds, which following a hashtag makes, with how many posts anybody may see carry each (`uses`), and the viewer's own. `q`, `sort` and paging as for all of them.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ListRequest{Sort: "name", Limit: 20}).
		Sends(http.StatusOK, libs_responses.List[*responses.HashtagNodeItem]{})
	hashtags.GET("/_browse/all", r.handler.Hashtag.All, r.setViewer).
		Summary("Every hashtag").
		Describe("Every hashtag on a post anybody may see, A to Z or the most used first, perhaps only those beginning with `q`: each with how many such posts carry it (`uses`), the viewer's own posts too, and whether the viewer follows it.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.ListRequest{Sort: "name", Limit: 20}).
		Sends(http.StatusOK, libs_responses.List[*responses.HashtagNodeItem]{})
	hashtags.GET("/_browse/ordered_by_degree", r.handler.Hashtag.BrowseOrderedByDegree, r.setViewer).
		Summary("Hashtags used most of all").
		Describe("Every hashtag, the ones in the most posts first.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewHashtagRequest().BrowseOrderedByDegree).
		Sends(http.StatusOK, libs_responses.List[*responses.HashtagNodeItem]{})
	// Offered to somebody writing (roadmap row 78). Signed in only:
	// writing needs it, and it is not a way to list every hashtag.
	hashtags.GET("/_suggest", r.handler.Hashtag.Begun, r.authorize).
		Summary("Hashtags that begin with what was typed").
		Describe("For somebody typing a hashtag: those in use that begin with `q` (what follows the #, its capitals and accents aside), the one that is just that first, then those in the viewer's custom feeds, then the most used. A hashtag is counted by the posts anybody may see, and the viewer's own.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.SuggestQuery{}).
		Sends(http.StatusOK, handlers.SuggestResponse{})
	hashtags.POST("/_suggest", r.handler.Hashtag.ForText, r.authorize).
		Summary("Hashtags for a text").
		Describe("For long content: the hashtags in use that are words of the title and body, or two words run together (open science, #openscience), and on more than one post. Those in the viewer's custom feeds first, then the words the text uses most. Hashtags the text has already are left out; addresses, code, mentions and hashtags are not words of it.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.SuggestRequest{}).
		Sends(http.StatusOK, handlers.SuggestResponse{})
	hashtags.GET("/", r.handler.Hashtag.Browse, r.setViewer).
		Summary("Hashtags").
		Viewer(apidoc.Anyone).
		Takes(requests.NewHashtagRequest().Browse).
		Sends(http.StatusOK, libs_responses.List[*responses.HashtagNodeItem]{})
	hashtags.GET("/:alias", r.handler.Hashtag.Read, r.setViewer).
		Summary("One hashtag").
		Viewer(apidoc.Anyone).
		Takes(requests.NewHashtagRequest().Read).
		Sends(http.StatusOK, responses.HashtagNodeItem{}).
		Refuses(http.StatusNotFound, "there is no such hashtag")

	// inbound
	hashtags.GET("/:id/inbounds", r.handler.Inbound.Browse, r.setViewer).
		Summary("The posts a hashtag is in").
		Describe("Those the viewer may see, the latest first unless asked otherwise.").
		Viewer(apidoc.Anyone).
		Takes(libs_requests.NewInboundRequest().Browse).
		Sends(http.StatusOK, libs_responses.List[*libs_responses.NodeItem]{}).
		Refuses(http.StatusNotFound, "there is no such hashtag")

	// admin — requires a valid OAuth access token.
	hashtags.DELETE("/:id", r.handler.Hashtag.Delete, r.authorize).
		Summary("Delete a hashtag").
		Describe("The posts it was in stay, their words unchanged.").
		Viewer(apidoc.Admin).
		Scope(libs_constants.OAuthScopeTaxonomyWrite).
		Takes(requests.NewHashtagRequest().Delete).
		SendsNothing(http.StatusOK, "the hashtag is gone").
		Refuses(http.StatusNotFound, "there is no such hashtag")

	return doc
}
