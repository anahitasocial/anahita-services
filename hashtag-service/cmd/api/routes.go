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
		Describe("The hashtags most used in a window of time that ends now. Not paged: the most there are, up to `limit`.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewHashtagRequest().BrowseTrending).
		Sends(http.StatusOK, []*responses.HashtagNodeItem{})
	hashtags.GET("/_browse/ordered_by_degree", r.handler.Hashtag.BrowseOrderedByDegree, r.setViewer).
		Summary("Hashtags used most of all").
		Describe("Every hashtag, the ones in the most posts first.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewHashtagRequest().BrowseOrderedByDegree).
		Sends(http.StatusOK, libs_responses.List[*responses.HashtagNodeItem]{})
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
	hashtags.PATCH("/:alias", r.handler.Hashtag.Edit, r.authorize).
		Summary("Change a hashtag").
		Viewer(apidoc.Admin).
		Scope(libs_constants.OAuthScopeTaxonomyWrite).
		Takes(requests.NewHashtagRequest().Edit).
		NotBuilt("a hashtag cannot be changed yet; it answers 501 whoever asks")
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
