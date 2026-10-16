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
}

func NewRoutes(
	handler *handlers.Handlers,
	setViewer echo.MiddlewareFunc,
) *Routes {
	return &Routes{
		handler:   handler,
		setViewer: setViewer,
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

	return doc
}
