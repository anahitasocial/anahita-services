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
		"The card of a link: what a page says of itself, read by this installation, with its "+
			"picture kept here. Optional: an installation may not run it (link-previews.md).")
	api := doc.On(e)

	// --- Link previews ---
	// Asking for a preview makes this service fetch a page, so it needs a
	// token (r.authorize). The group carries no middleware of its own:
	// each route names its own.
	previews := api.Group("link-previews")

	// read describes the preview route, which is registered twice, with
	// and without the trailing slash.
	read := func(route *apidoc.Route) {
		route.
			Summary("The preview of a link").
			Describe("Read from the page by this installation when no fresh preview is kept, so "+
				"that nobody's browser asks the other site. For the composer, while a post is "+
				"written.").
			Viewer(apidoc.SignedIn).
			Takes(&handlers.PreviewRequest{}).
			Sends(http.StatusOK, handlers.PreviewResponse{}).
			SendsNothing(http.StatusNoContent, "there is nothing to show: the page could not be "+
				"read, is not a page, or says nothing of itself").
			Refuses(http.StatusTooManyRequests, "the viewer has asked for too many lately")
	}

	previews.GET("", r.handler.Previews.Read, r.authorize).With(read)

	previews.GET("/", r.handler.Previews.Read, r.authorize).With(read)

	// --- For the other services ---
	// For the other services of this installation, and not for anybody
	// outside it: the gateway sends nothing here, since its address is not
	// under /link-previews. No middleware, so the caller is never known.
	// See ReadInternal.
	api.GET("/internal/link-previews", r.handler.Previews.ReadInternal).
		Summary("The preview of a link, for another service").
		Internal("activitypub-service asks it for the card of a link in a post that came from "+
			"another server.").
		Describe("The preview as it is kept, the key of its picture included, which the public "+
			"route does not give out. A request that came through the gateway is answered 404.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.PreviewRequest{}).
		Sends(http.StatusOK, linkpreviews.Preview{}).
		SendsNothing(http.StatusNoContent, "there is nothing to show").
		Refuses(http.StatusNotFound, "it came through the gateway").
		Refuses(http.StatusTooManyRequests, "the services have asked for too many lately")

	// --- Pictures ---
	// A preview's picture is read by whoever reads the note it is on, a
	// visitor included where the installation lets visitors read. Public:
	// no token is needed, and when one is sent the caller is known
	// (r.setViewer).
	previews.GET("/image/:name", r.handler.Previews.Image, r.setViewer).
		Summary("A preview's picture").
		Describe("The picture itself. Its name is the hash of what is in it, so it never changes "+
			"and may be kept for a year.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.ImageRequest{}).
		Media("image/jpeg").
		Sends(http.StatusOK, []byte{},
			"the picture: a JPEG, or a PNG, GIF or WebP as its name says").
		Refuses(http.StatusNotFound, "there is no such picture")

	return doc
}
