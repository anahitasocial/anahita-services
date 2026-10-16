package main

import (
	"net/http"

	"identityimage/cmd/api/handlers"

	"anahita-libs/apidoc"
	"anahita-libs/data/entities"

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
	for _, kind := range []struct {
		path    string
		handler echo.HandlerFunc
		what    string
	}{
		{"/avatars/:uuid/:size", r.handler.Image.PublicAvatar, "A public profile's avatar"},
		{"/covers/:uuid/:size", r.handler.Image.PublicCover, "A public profile's or post's cover"},
	} {
		api.GET(kind.path, kind.handler, r.setViewer).
			Summary(kind.what+", at an address that does not change").
			Describe("For other servers, which keep the address of a picture for weeks: a redirect to a freshly signed address in storage. Only what anybody may see is answered for; anything else, on a site for members included, is not found, in the same words as what does not exist.").
			Viewer(apidoc.Anyone).
			Takes(&handlers.PublicImageRequest{}).
			SendsNothing(http.StatusFound, "the picture is at the address in Location").
			Refuses(http.StatusNotFound, "there is no such picture that anybody may see")
	}

	// Each kind of image keeps the address it had when it was a service of
	// its own, and the gateway sends both here. Every route needs a valid
	// OAuth access token; whether that person may change this node is
	// decided in the handler.
	for _, kind := range []struct {
		group  string
		set    echo.HandlerFunc
		remove echo.HandlerFunc
		upload any
		what   string
		on     string
	}{
		{"avatars", r.handler.Image.SetAvatar, r.handler.Image.RemoveAvatar, &handlers.AvatarUpload{}, "avatar", "a profile"},
		{"covers", r.handler.Image.SetCover, r.handler.Image.RemoveCover, &handlers.CoverUpload{}, "cover", "a profile or a post"},
	} {
		group := api.Group(kind.group, r.authorize)
		group.POST("/:node_id/", kind.set).
			Summary("Set the "+kind.what+" of "+kind.on).
			Describe("Replaces any there was, which is removed from storage. Answers with each size of the new one, at addresses good for a short while. Note that its fields are named as Go names them (`Url`, `Size`), not in snake_case.").
			Viewer(apidoc.SignedIn).
			Takes(kind.upload).
			Sends(http.StatusCreated, map[string]entities.AnahitaNodeImage{}).
			Refuses(http.StatusBadRequest, "`file_required` when no file was sent, `file_unreadable` when it cannot be read as a picture").
			Refuses(http.StatusForbidden, "it cannot have one, or the viewer may not change it").
			Refuses(http.StatusNotFound, "there is no such profile or post, or the viewer may not see it").
			Refuses(http.StatusRequestEntityTooLarge, "`file_too_large`: more than 10 MB, or too many pixels").
			Refuses(http.StatusUnsupportedMediaType, "`file_unsupported`: not a JPEG or a PNG")
		group.DELETE("/:node_id/", kind.remove).
			Summary("Remove the "+kind.what+" of "+kind.on).
			Viewer(apidoc.SignedIn).
			Takes(&handlers.NodeImageRequest{}).
			SendsNothing(http.StatusOK, "it has none now, whether or not it had one").
			Refuses(http.StatusForbidden, "it cannot have one, or the viewer may not change it").
			Refuses(http.StatusNotFound, "there is no such profile or post, or the viewer may not see it")
	}

	return doc
}
