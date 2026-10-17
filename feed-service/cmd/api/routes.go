package main

import (
	"net/http"

	"feed/cmd/api/handlers"
	"feed/requests"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
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
// the descriptions: docs/api/feed-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("feed-service", "Feeds",
		"The home feed and a profile's posts, custom feeds that a person makes and names (feeds.md), and reposts.")
	api := doc.On(e)
	definition := "why the definition was refused: `malformed`, `unknown_field`, `too_broad`, `too_many_actors`, `too_many_hashtags`, `too_many_keywords`, `too_many_locations`, `keyword_length`, `invalid_hashtag`, `unknown_media_type`, `unknown_actor_type`, `invalid_actor`, `invalid_location`, `invalid_language`, `invalid_near` (an area's point or radius, 1 to 500 km), `unknown_origin` or `unknown_order`; and, reading a feed of what is near its reader with no `near_latitude` and `near_longitude`, `location_needed`; `invalid` for a field"
	feedList := responses.Unpaged[*handlers.CustomFeedResponse]{}

	// feeds — public, but still identify the caller when a token is
	// present so the viewer drives access filtering.
	feeds := api.Group("feeds")
	feeds.GET("/leaders/", r.handler.FeedLeaders.Browse, r.setViewer).
		Summary("The home feed").
		Describe("Posts from the profiles the viewer follows, the latest first. In order of time, not ranked: a ranked feed is a custom feed the viewer makes.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewFeedLeadersRequest().Browse).
		Sends(http.StatusOK, responses.List[*responses.NodeItem]{})
	feeds.GET("/actor/:actor_id/", r.handler.FeedActor.Browse, r.setViewer).
		Summary("A profile's posts, replies or reposts").
		Describe("The ones the viewer may see, its pin first among its posts.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewFeedActorRequest().Browse).
		Sends(http.StatusOK, responses.List[*responses.NodeItem]{}).
		Sends(http.StatusUnauthorized, responses.Refusal{}, "`members_only`: replies are not part of a preview").
		Refuses(http.StatusForbidden, "the viewer may not see the profile").
		Refuses(http.StatusNotFound, "there is no such profile")

	// Custom feeds. Making, changing and keeping one need somebody signed
	// in; reading one does not, when it is public. The fixed paths are
	// matched before ":id".
	custom := r.handler.CustomFeed
	feeds.POST("/", custom.Add, r.authorize).
		Summary("Make a custom feed").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeMediaWrite).
		Takes(&handlers.CustomFeedFields{}).
		Sends(http.StatusCreated, handlers.CustomFeedResponse{}).
		Refuses(http.StatusForbidden, "the viewer may not make one").
		Sends(http.StatusConflict, responses.Refusal{}, "`too_many_feeds`").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, definition)
	feeds.GET("/mine", custom.Mine, r.authorize).
		Summary("The viewer's custom feeds").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, feedList)
	feeds.GET("/pinned", custom.Pinned, r.authorize).
		Summary("The feeds on the viewer's home page").
		Describe("Theirs and other people's they keep there, in order.").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, feedList)
	feeds.GET("/discover", custom.Discover, r.setViewer).
		Summary("Feeds people have shared").
		Describe("The ones the viewer may see, the newest first. Paged by how many came before, with no total.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.CustomFeedDiscoverRequest{}).
		Sends(http.StatusOK, handlers.DiscoverPage{})
	feeds.PUT("/pins", custom.SetPins, r.authorize).
		Summary("Set the feeds on the viewer's home page").
		Describe("The whole list, in order. Answers with it.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.CustomFeedPinsRequest{}).
		Sends(http.StatusOK, feedList).
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, "`too_many_pins`; or `feed_not_found`, one the viewer may not see")
	feeds.POST("/preview", custom.Preview, r.authorize).
		Summary("Try a definition").
		Describe("The first page of what a feed of this definition would show, before it is made.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeMediaWrite).
		Takes(&handlers.CustomFeedPreviewRequest{}).
		Sends(http.StatusOK, handlers.CustomFeedPage{}).
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, definition)
	feeds.GET("/:id", custom.Read, r.setViewer).
		Summary("One custom feed").
		Describe("With the profiles and places its definition names, the ones the viewer may see, in `references`.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.CustomFeedIDRequest{}).
		Sends(http.StatusOK, handlers.CustomFeedResponse{}).
		Refuses(http.StatusNotFound, "there is no such feed that the viewer may see")
	feeds.PATCH("/:id", custom.Edit, r.authorize).
		Summary("Change a custom feed").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeMediaWrite).
		Takes(&handlers.CustomFeedEditRequest{}).
		Sends(http.StatusOK, handlers.CustomFeedResponse{}).
		Refuses(http.StatusForbidden, "it is not the viewer's").
		Refuses(http.StatusNotFound, "there is no such feed").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, definition)
	feeds.DELETE("/:id", custom.Delete, r.authorize).
		Summary("Delete a custom feed").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeMediaWrite).
		Takes(&handlers.CustomFeedIDRequest{}).
		SendsNothing(http.StatusNoContent, "it is gone").
		Refuses(http.StatusForbidden, "it is not the viewer's").
		Refuses(http.StatusNotFound, "there is no such feed")
	feeds.GET("/:id/items", custom.Items, r.setViewer).
		Summary("A page of a custom feed").
		Describe("What it shows, as the viewer may see it, the latest first. Paged by what came before: the next page is asked for with the `next_cursor` of this one.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.CustomFeedItemsRequest{}).
		Sends(http.StatusOK, handlers.CustomFeedPage{}).
		Refuses(http.StatusNotFound, "there is no such feed that the viewer may see").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, "`malformed`: the cursor is not one")
	feeds.POST("/:id/copy", custom.Copy, r.authorize).
		Summary("Copy a custom feed").
		Describe("A feed of the viewer's own with the same definition, to change.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeMediaWrite).
		Takes(&handlers.CustomFeedCopyRequest{}).
		Sends(http.StatusCreated, handlers.CustomFeedResponse{}).
		Refuses(http.StatusForbidden, "the viewer may not copy it").
		Refuses(http.StatusNotFound, "there is no such feed that the viewer may see").
		Sends(http.StatusConflict, responses.Refusal{}, "`too_many_feeds`").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, "`malformed`")
	pin := func(what string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.
				Summary(what).
				Viewer(apidoc.SignedIn).
				Takes(&handlers.CustomFeedIDRequest{}).
				SendsNothing(http.StatusNoContent, "done").
				Refuses(http.StatusNotFound, "there is no such feed that the viewer may see").
				Sends(http.StatusConflict, responses.Refusal{}, "`too_many_pins`")
		}
	}
	feeds.PUT("/:id/pin", custom.Pin, r.authorize).With(pin("Keep a feed on the viewer's home page"))
	feeds.DELETE("/:id/pin", custom.Unpin, r.authorize).With(pin("Take it off the viewer's home page"))

	// reposts — every route requires a valid OAuth access token.
	reposts := api.Group("reposts/", r.authorize)
	reposts.GET(":parent_id/", r.handler.Repost.Browse).
		Summary("Who reposted a post").
		Viewer(apidoc.SignedIn).
		Takes(requests.NewRepostRequest().Browse).
		Sends(http.StatusOK, responses.List[*responses.ActorNodeItem]{}).
		Refuses(http.StatusForbidden, "the viewer may not see it")
	reposts.POST(":parent_id/", r.handler.Repost.Add).
		Summary("Repost a post").
		Describe("Answers with the repost.").
		Viewer(apidoc.SignedIn).
		Takes(requests.NewRepostRequest().Add).
		Sends(http.StatusCreated, responses.NodeItem{}).
		Refuses(http.StatusForbidden, "the viewer may not repost it")
	reposts.DELETE(":parent_id/", r.handler.Repost.Delete).
		Summary("Undo a repost").
		Viewer(apidoc.SignedIn).
		Takes(requests.NewRepostRequest().Delete).
		SendsNothing(http.StatusOK, "undone").
		Refuses(http.StatusForbidden, "the viewer may not undo this repost")

	return doc
}
