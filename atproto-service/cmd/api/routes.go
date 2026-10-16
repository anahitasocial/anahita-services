package main

import (
	"net/http"

	"atproto/cmd/api/handlers"

	"anahita-libs/apidoc"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
}

func NewRoutes(handler *handlers.Handlers, authorize echo.MiddlewareFunc) *Routes {
	return &Routes{handler: handler, authorize: authorize}
}

func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("atproto-service", "Bluesky",
		"Finding and following people on Bluesky, and a person's own Bluesky account connected, which what they do here is written as. Optional: an installation may not run it (federation.md, Bluesky).")
	api := doc.On(e)
	atproto := api.Group("atproto", r.authorize)
	off := "`bluesky_writing_off`: this installation does not write to Bluesky"

	// Finding somebody on Bluesky.
	atproto.GET("/lookup", r.handler.Lookup).
		Summary("Find somebody on Bluesky").
		Describe("They are kept here, so that they can be followed as anybody is. Thirty lookups in ten minutes.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.LookupRequest{}).
		Sends(http.StatusOK, handlers.RemotePerson{}).
		Refuses(http.StatusBadRequest, "`lookup_unreadable`: not something that names an account").
		Refuses(http.StatusNotFound, "`lookup_not_found`, there is nobody by that name; `lookup_is_here`, it is somebody of this installation; `lookup_refused`, Bluesky or their name is refused here").
		Refuses(http.StatusTooManyRequests, "`too_many_lookups`").
		Refuses(http.StatusBadGateway, "`lookup_failed`: Bluesky could not be asked")
	// The pictures of a post from Bluesky, copied here again after they
	// were removed for its age.
	atproto.POST("/posts/:id/pictures", r.handler.RestorePictures).
		Summary("Copy a post's pictures here again").
		Describe("The pictures of a post from Bluesky that has been left alone a while are removed from storage, and the post says so (`media_removed`). Asked here, they are read again from Bluesky and copied as they were. For anybody signed in who may see the post.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.PostPicturesRequest{}).
		Sends(http.StatusOK, handlers.PicturesRestored{}).
		Refuses(http.StatusNotFound, "`media_not_removed`: there is no such post the viewer may see, or its pictures are here").
		Refuses(http.StatusGone, "`media_gone`: they are not on Bluesky any more").
		Refuses(http.StatusTooManyRequests, "`too_many_requests`").
		Refuses(http.StatusBadGateway, "`media_failed`: Bluesky could not be asked")
	// A person's own Bluesky account, connected or not.
	atproto.GET("/settings", r.handler.ReadSettings).
		Summary("The viewer's Bluesky account").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.Settings{}).
		Refuses(http.StatusNotFound, off)
	atproto.PUT("/settings", r.handler.EditSettings).
		Summary("Connect the viewer's Bluesky account").
		Describe("With an app password, which is exchanged for a token and not kept. Ten tries in ten minutes.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ConnectRequest{}).
		Sends(http.StatusOK, handlers.Settings{}).
		Refuses(http.StatusBadRequest, "`bluesky_unreadable`").
		Refuses(http.StatusForbidden, "`bluesky_denied`: the password is not taken").
		Refuses(http.StatusNotFound, off+"; `bluesky_not_found`, there is no such account; `lookup_refused`, Bluesky is refused here").
		Refuses(http.StatusConflict, "`bluesky_taken`: somebody else here connected that account").
		Refuses(http.StatusTooManyRequests, "`too_many_tries`").
		Refuses(http.StatusBadGateway, "`bluesky_failed`")
	atproto.DELETE("/settings", r.handler.DeleteSettings).
		Summary("Disconnect it").
		Describe("The token is forgotten. What was written stays on Bluesky.").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.Settings{}).
		Refuses(http.StatusNotFound, off)
	// Following here whoever they follow on Bluesky already.
	atproto.GET("/settings/import-follows", r.handler.ImportPreview).
		Summary("What following here whoever the viewer follows on Bluesky would do").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.ImportPreview{}).
		Refuses(http.StatusNotFound, off).
		Refuses(http.StatusConflict, "`bluesky_not_connected`").
		Refuses(http.StatusBadGateway, "`bluesky_failed`")
	atproto.POST("/settings/import-follows", r.handler.ImportFollows).
		Summary("Follow here whoever the viewer follows on Bluesky").
		Describe("The ones followed most lately first, leaving out whoever is refused or blocked. Answered at once and carried on with afterwards. Nothing is written to Bluesky. Three times an hour.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ImportRequest{}).
		Sends(http.StatusAccepted, handlers.ImportStarted{}).
		Refuses(http.StatusNotFound, off).
		Refuses(http.StatusConflict, "`bluesky_not_connected`").
		Refuses(http.StatusTooManyRequests, "`too_many_tries`").
		Refuses(http.StatusBadGateway, "`bluesky_failed`")

	return doc
}
