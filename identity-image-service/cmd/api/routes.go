package main

import (
	"net/http"

	"identityimage/cmd/api/handlers"

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
// the descriptions: docs/api/identity-image-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("identity-image-service", "Avatars and covers",
		"The picture that stands for a profile, and the wide picture across the top of a profile or a post.")
	api := doc.On(e)

	// A public profile's pictures at an address that does not change, for
	// other servers to keep: by the node's UUID, answered with a redirect to
	// a freshly signed address. No token is needed, and whoever is asking
	// makes no difference: see handlers/public.go for what is answered for.
	public := func(what string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.
				Summary(what+", at an address that does not change").
				Describe("For other servers, which keep the address of a picture for weeks: a redirect to a freshly signed address in storage. Only what anybody may see is answered for; anything else, on a site for members included, is not found, in the same words as what does not exist.").
				Viewer(apidoc.Anyone).
				Takes(&handlers.PublicImageRequest{}).
				SendsNothing(http.StatusFound, "the picture is at the address in Location").
				Refuses(http.StatusNotFound, "there is no such picture that anybody may see")
		}
	}
	api.GET("/avatars/:uuid/:size", r.handler.Image.PublicAvatar, r.setViewer).With(public("A public profile's avatar"))
	api.GET("/covers/:uuid/:size", r.handler.Image.PublicCover, r.setViewer).With(public("A public profile's or post's cover"))

	// Each kind of image keeps the address it had when it was a service of
	// its own, and the gateway sends both here. Every route needs a valid
	// OAuth access token; whether that person may change this node is
	// decided in the handler.
	set := func(what, on string, upload any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.
				Summary("Set the "+what+" of "+on).
				Describe("Replaces any there was, which is removed from storage. Answers with each size of the new one, at addresses good for a short while.").
				Viewer(apidoc.SignedIn).
				Takes(upload).
				Sends(http.StatusCreated, map[string]responses.Image{}).
				Sends(http.StatusBadRequest, responses.Refusal{}, "`file_required` when no file was sent, `file_unreadable` when it cannot be read as a picture").
				Refuses(http.StatusForbidden, "it cannot have one, or the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such profile or post, or the viewer may not see it").
				Sends(http.StatusRequestEntityTooLarge, responses.Refusal{}, "`file_too_large`: more than 10 MB, or too many pixels").
				Sends(http.StatusUnsupportedMediaType, responses.Refusal{}, "`file_unsupported`: not a JPEG or a PNG")
		}
	}
	remove := func(what, on string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.
				Summary("Remove the "+what+" of "+on).
				Viewer(apidoc.SignedIn).
				Takes(&handlers.NodeImageRequest{}).
				SendsNothing(http.StatusOK, "it has none now, whether or not it had one").
				Refuses(http.StatusForbidden, "it cannot have one, or the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such profile or post, or the viewer may not see it")
		}
	}

	avatars := api.Group("avatars", r.authorize)
	avatars.POST("/:node_id/", r.handler.Image.SetAvatar).With(set("avatar", "a profile", &handlers.AvatarUpload{}))
	avatars.DELETE("/:node_id/", r.handler.Image.RemoveAvatar).With(remove("avatar", "a profile"))

	covers := api.Group("covers", r.authorize)
	covers.POST("/:node_id/", r.handler.Image.SetCover).With(set("cover", "a profile or a post", &handlers.CoverUpload{}))
	covers.DELETE("/:node_id/", r.handler.Image.RemoveCover).With(remove("cover", "a profile or a post"))

	return doc
}
