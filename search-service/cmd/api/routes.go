package main

import (
	"net/http"

	"search/cmd/api/handlers"
	"search/requests"

	"anahita-libs/apidoc"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	setViewer echo.MiddlewareFunc
	authorize echo.MiddlewareFunc
}

func NewRoutes(
	handler *handlers.Handlers,
	setViewer echo.MiddlewareFunc,
	authorize echo.MiddlewareFunc,
) *Routes {
	return &Routes{
		handler:   handler,
		setViewer: setViewer,
		authorize: authorize,
	}
}

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/search-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("search-service", "Search",
		"Finding posts, profiles and custom feeds by their words.")
	api := doc.On(e)

	// search — public, but still identifies the caller when a token is
	// present so the viewer drives access filtering.
	search := api.Group("search")
	search.GET("/", r.handler.Search.Browse, r.setViewer).
		Summary("Search").
		Describe("Posts, profiles and custom feeds whose words match, of those the viewer may see. Search applies a post's audience and its profile's, but not the exceptions that let a profile's owner and a post's author see their own (permissions.md), so it may not find one's own restricted posts. Each person, group and event found says how it stands with the viewer, and each event when it is.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewSearchRequest().Browse).
		Sends(http.StatusOK, responses.List[*responses.NodeItem]{})

	// The viewer's own weights for Most relevant, as they have their own
	// for recommendations.
	search.GET("/preferences", r.handler.Preferences.Read, r.authorize).
		Summary("The viewer's weights for Most relevant").
		Describe("How much each sign that something was found useful counts for the viewer: a step that multiplies the installation's own weight (search.md).").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.PreferencesResponse{})
	search.PUT("/preferences", r.handler.Preferences.Edit, r.authorize).
		Summary("Change the viewer's weights for Most relevant").
		Describe("What is sent replaces what there was; a name left out is as the installation has it. Answers with them.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.PreferencesRequest{}).
		Sends(http.StatusOK, handlers.PreferencesResponse{}).
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, "`invalid_weight`: a name that may not be set, or a value that is not a step; nothing is changed")

	return doc
}
