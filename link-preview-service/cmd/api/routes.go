package main

import (
	"net/http"

	"linkpreview/cmd/api/handlers"

	"anahita-libs/apidoc"
	"anahita-libs/linkpreviews"

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
// the descriptions: docs/api/link-preview-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("link-preview-service", "Link previews",
		"The card of a link: what a page says of itself, read by this installation, with its picture kept here. Optional: an installation may not run it (link-previews.md).")
	api := doc.On(e)
	previews := api.Group("link-previews")

	// Asking for a preview makes this service fetch a page: somebody
	// signed in only.
	read := func(route *apidoc.Route) {
		route.
			Summary("The preview of a link").
			Describe("Read from the page by this installation when no fresh preview is kept, so that nobody's browser asks the other site. For the composer, while a post is written.").
			Viewer(apidoc.SignedIn).
			Takes(&handlers.PreviewRequest{}).
			Sends(http.StatusOK, handlers.PreviewResponse{}).
			SendsNothing(http.StatusNoContent, "there is nothing to show: the page could not be read, is not a page, or says nothing of itself").
			Refuses(http.StatusTooManyRequests, "the viewer has asked for too many lately")
	}
	previews.GET("", r.handler.Previews.Read, r.authorize).With(read)
	previews.GET("/", r.handler.Previews.Read, r.authorize).With(read)

	// For the other services of this installation, and not for anybody
	// outside it: the gateway sends nothing here, since its address is not
	// under /link-previews. activitypub-service asks it for the card of a
	// link in a post that came from another server. See ReadInternal.
	api.GET("/internal/link-previews", r.handler.Previews.ReadInternal).
		Summary("The preview of a link, for another service").
		Internal("activitypub-service asks it for the card of a link in a post that came from another server.").
		Describe("The preview as it is kept, the key of its picture included, which the public route does not give out. A request that came through the gateway is answered 404.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.PreviewRequest{}).
		Sends(http.StatusOK, linkpreviews.Preview{}).
		SendsNothing(http.StatusNoContent, "there is nothing to show").
		Refuses(http.StatusNotFound, "it came through the gateway").
		Refuses(http.StatusTooManyRequests, "the services have asked for too many lately")

	// A preview's picture is read by whoever reads the note it is on, a
	// visitor included where the installation lets visitors read.
	previews.GET("/image/:name", r.handler.Previews.Image, r.setViewer).
		Summary("A preview's picture").
		Describe("A redirect to the picture in storage, at an address good for a short while.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.ImageRequest{}).
		SendsNothing(http.StatusFound, "the picture is at the address in Location").
		Refuses(http.StatusNotFound, "there is no such picture")

	return doc
}
