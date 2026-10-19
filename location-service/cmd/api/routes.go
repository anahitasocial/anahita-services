package main

import (
	"net/http"

	"location/cmd/api/handlers"
	"location/requests"
	"location/responses"

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

func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("location-service", "Locations",
		"Locations posts can be tied to: placed on a map, or found from an address, and the posts tied to each.")
	api := doc.On(e)
	taxonomy := libs_constants.OAuthScopeTaxonomyWrite
	media := libs_constants.OAuthScopeMediaWrite

	// locations — the read routes are public but still identify the
	// caller when a token is present, so the viewer drives access
	// filtering.
	locations := api.Group("locations")
	locations.GET("/_browse/trending", r.handler.Location.BrowseTrending, r.setViewer).
		Summary("Locations used most lately").
		Describe("The locations the most people tied a post to in a window of time that ends now: each person once, public posts only, by when they were written. `time_window` left out, the installation's own (`TRENDING_WINDOW`; daily where it federates, monthly where not), widened while it holds fewer than ten; `window` says which. One page, all there is: up to `limit`.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewLocationRequest().BrowseTrending).
		Sends(http.StatusOK, libs_responses.Trending[*responses.LocationNodeItem]{})
	locations.GET("/_browse/ordered_by_degree", r.handler.Location.BrowseOrderedByDegree, r.setViewer).
		Summary("Locations used most of all").
		Viewer(apidoc.Anyone).
		Takes(requests.NewLocationRequest().BrowseOrderedByDegree).
		Sends(http.StatusOK, libs_responses.List[*responses.LocationNodeItem]{})
	locations.GET("/", r.handler.Location.Browse, r.setViewer).
		Summary("Locations").
		Describe("Found by words, near a point, or the locations one post is tied to.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewLocationRequest().Browse).
		Sends(http.StatusOK, libs_responses.List[*responses.LocationNodeItem]{})
	locations.GET("/:id", r.handler.Location.Read, r.setViewer).
		Summary("One location").
		Viewer(apidoc.Anyone).
		Takes(requests.NewLocationRequest().Read).
		Sends(http.StatusOK, responses.LocationNodeItem{}).
		Refuses(http.StatusNotFound, "there is no such location")

	// inbound — browsing is public like the reads above; tagging and
	// untagging are writes and require a token. They previously sat on
	// setViewer only, which was a leftover from an older route table
	// rather than a decision, and left both open to anonymous callers.
	locations.GET("/:id/inbounds", r.handler.Inbound.Browse, r.setViewer).
		Summary("The posts tied to a location").
		Describe("Those the viewer may see.").
		Viewer(apidoc.Anyone).
		Takes(libs_requests.NewInboundRequest().Browse).
		Sends(http.StatusOK, libs_responses.List[*libs_responses.NodeItem]{}).
		Refuses(http.StatusNotFound, "there is no such location")
	locations.POST("/:id/inbounds/", r.handler.Inbound.Add, r.authorize).
		Summary("Tie a post to a location").
		Describe("For the post's author, the owner of the profile it is on, and site administrators.").
		Viewer(apidoc.SignedIn).
		Scope(media).
		Takes(libs_requests.NewInboundRequest().Add).
		SendsNothing(http.StatusCreated, "tied, or tied already").
		Refuses(http.StatusForbidden, "the viewer may not change the post").
		Refuses(http.StatusNotFound, "there is no such location or post").
		Sends(http.StatusUnprocessableEntity, libs_responses.Refusal{}, "`too_many_locations`: the post is tied to five locations already")
	locations.DELETE("/:id/inbounds/:source_id", r.handler.Inbound.Delete, r.authorize).
		Summary("Untie a post from a location").
		Describe("For the post's author, the owner of the profile it is on, and site administrators.").
		Viewer(apidoc.SignedIn).
		Scope(media).
		Takes(libs_requests.NewInboundRequest().Delete).
		SendsNothing(http.StatusOK, "untied").
		Refuses(http.StatusForbidden, "the viewer may not change the post").
		Refuses(http.StatusNotFound, "there is no such location or post")

	// admin — requires a valid OAuth access token.
	locations.PATCH("/:id", r.handler.Location.Edit, r.authorize).
		Summary("Change a location").
		Describe("A point given (`latitude`, `longitude`) is kept as it is; without one, a changed address is found on the map again, and clearing the address keeps the point. Answers with the location.").
		Viewer(apidoc.Admin).
		Scope(taxonomy).
		Takes(requests.NewLocationRequest().Edit).
		Sends(http.StatusOK, responses.LocationNodeItem{}).
		Refuses(http.StatusBadRequest, "not well formed, or the address could not be found on a map").
		Refuses(http.StatusNotFound, "there is no such location")
	locations.POST("/", r.handler.Location.Add, r.authorize).
		Summary("Add a location").
		Describe("It needs a point: one given (`latitude`, `longitude`), placed on a map, or an address to find it by. The address is optional, since a beach or a trail has none. Answers with the location.").
		Viewer(apidoc.Admin).
		Scope(taxonomy).
		Takes(requests.NewLocationRequest().Add).
		Sends(http.StatusCreated, responses.LocationNodeItem{}).
		Refuses(http.StatusBadRequest, "not well formed, or the address could not be found on a map")
	locations.DELETE("/:id", r.handler.Location.Delete, r.authorize).
		Summary("Delete a location").
		Viewer(apidoc.Admin).
		Scope(taxonomy).
		Takes(requests.NewLocationRequest().Delete).
		SendsNothing(http.StatusOK, "it is gone").
		Refuses(http.StatusNotFound, "there is no such location")

	return doc
}
