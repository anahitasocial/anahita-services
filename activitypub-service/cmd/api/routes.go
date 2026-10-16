package main

import (
	"encoding/json"
	"net/http"

	"activitypub/ap"
	"activitypub/cmd/api/handlers"

	"anahita-libs/apidoc"
	"anahita-libs/federation"
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
// the descriptions: docs/api/activitypub-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("activitypub-service", "The fediverse",
		"This installation on the fediverse, by ActivityPub: what other servers ask of it and send to it, a person's choice to be on it, finding somebody elsewhere, and the servers it refuses (federation.md). Optional: an installation may not run it.")
	api := doc.On(e)
	activity := "application/activity+json"
	server := "For other servers, in ActivityStreams (https://www.w3.org/TR/activitystreams-vocabulary/). "
	gone := "nobody here by that name is on the fediverse"

	// What other servers ask. No token, since a server has none: who is
	// answered for is decided in each handler, and it is only ever somebody
	// who chose to be on the fediverse. setViewer is on each because it is
	// what answers for a site that is for members only.
	api.GET("/.well-known/webfinger", r.handler.WebFinger, r.setViewer).
		Summary("Who an account is").
		Describe("WebFinger (RFC 7033): where another server finds somebody here by their handle.").
		Viewer(apidoc.Anyone).
		Media("application/jrd+json").
		Takes(&handlers.WebFingerRequest{}).
		Unbound().
		Sends(http.StatusOK, ap.JRD{}).
		Refuses(http.StatusBadRequest, "no resource, or not one that is asked about").
		Refuses(http.StatusNotFound, gone)
	api.GET("/.well-known/host-meta", r.handler.HostMeta, r.setViewer).
		Summary("Where WebFinger is").
		Describe("An XRD document (RFC 6415), for servers that look for WebFinger this way.").
		Viewer(apidoc.Anyone).
		Media("application/xrd+xml").
		Sends(http.StatusOK, "", "the XRD document")

	// Where other servers send what somebody did: one for the whole
	// installation, and one for each person. They are the same door.
	inbox := func(route *apidoc.Route) {
		route.
			Summary("Take an activity from another server").
			Describe(server+"Signed with HTTP Signatures; what is not signed by whoever is said to have done it is not believed. Six hundred a minute from one address. Taken, whether or not anything is done with it.").
			Viewer(apidoc.Anyone).
			Media(activity).
			TakesBody(json.RawMessage{}).
			SendsNothing(http.StatusAccepted, "taken").
			Refuses(http.StatusUnauthorized, "not signed, or the signature does not check, or not by whoever is said to have done it").
			Refuses(http.StatusForbidden, "the server it came from is refused").
			Refuses(http.StatusNotFound, gone).
			Refuses(http.StatusRequestEntityTooLarge, "too large").
			Refuses(http.StatusTooManyRequests, "too many from one address").
			Refuses(http.StatusServiceUnavailable, "it could not be kept now; send it again")
	}
	api.POST("/inbox", r.handler.Inbox, r.setViewer).With(inbox)

	ap_ := api.Group("ap")
	ap_.POST("/users/:uuid/inbox", r.handler.Inbox, r.setViewer).
		With(inbox).
		Takes(&handlers.ActorRef{}).
		Unbound()
	ap_.GET("/actor", r.handler.InstanceActor, r.setViewer).
		Summary("The installation itself").
		Describe(server+"The actor that signs what the installation asks of other servers.").
		Viewer(apidoc.Anyone).
		Media(activity).
		Sends(http.StatusOK, ap.Actor{}).
		Refuses(http.StatusNotFound, "the installation is not on the fediverse").
		Refuses(http.StatusServiceUnavailable, "its key cannot be read now")
	ap_.GET("/users/:uuid", r.handler.Actor, r.setViewer).
		Summary("Somebody here, as other servers see them").
		Describe(server+"A `Person`, with their public key.").
		Viewer(apidoc.Anyone).
		Media(activity).
		Takes(&handlers.ActorRef{}).
		Unbound().
		Sends(http.StatusOK, ap.Actor{}).
		Refuses(http.StatusNotFound, gone).
		Refuses(http.StatusServiceUnavailable, "their key cannot be read now")
	ap_.GET("/users/:uuid/followers", r.handler.Followers, r.setViewer).
		Summary("How many follow them").
		Describe(server+"A collection with its count and no items.").
		Viewer(apidoc.Anyone).
		Media(activity).
		Takes(&handlers.ActorRef{}).
		Unbound().
		Sends(http.StatusOK, ap.Collection{}).
		Refuses(http.StatusNotFound, gone)
	ap_.GET("/users/:uuid/following", r.handler.Following, r.setViewer).
		Summary("How many they follow").
		Describe(server+"A collection with its count and no items.").
		Viewer(apidoc.Anyone).
		Media(activity).
		Takes(&handlers.ActorRef{}).
		Unbound().
		Sends(http.StatusOK, ap.Collection{}).
		Refuses(http.StatusNotFound, gone)
	ap_.GET("/users/:uuid/outbox", r.handler.Outbox, r.setViewer).
		Summary("How many posts they have sent out").
		Describe(server+"A collection with its count and no items.").
		Viewer(apidoc.Anyone).
		Media(activity).
		Takes(&handlers.ActorRef{}).
		Unbound().
		Sends(http.StatusOK, ap.Collection{}).
		Refuses(http.StatusNotFound, gone).
		Refuses(http.StatusServiceUnavailable, "it cannot be counted now")
	ap_.GET("/notes/:uuid", r.handler.Note, r.setViewer).
		Summary("A post, as other servers see it").
		Describe(server+"A `Note`, of somebody on the fediverse, open to everybody.").
		Viewer(apidoc.Anyone).
		Media(activity).
		Takes(&handlers.NoteRef{}).
		Unbound().
		Sends(http.StatusOK, ap.Note{}).
		Refuses(http.StatusNotFound, "there is no such post that went out")
	// One of a photo post's pictures, for a server that was sent the post.
	ap_.GET("/notes/:uuid/pictures/:position", r.handler.Picture, r.setViewer).
		Summary("A picture of a post").
		Describe("For a server that was sent the post: a redirect to the picture in storage.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.NotePictureRef{}).
		Unbound().
		SendsNothing(http.StatusFound, "the picture is at the address in Location").
		Refuses(http.StatusNotFound, "there is no such picture of a post that went out")
	// A video post's file, and its poster, for a server that was sent it.
	ap_.GET("/notes/:uuid/video", r.handler.Video, r.setViewer).
		Summary("The video of a post").
		Describe("For a server that was sent the post: a redirect to the file in storage.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.NoteRef{}).
		Unbound().
		SendsNothing(http.StatusFound, "the file is at the address in Location").
		Refuses(http.StatusNotFound, "there is no such video of a post that went out")
	ap_.GET("/notes/:uuid/poster", r.handler.VideoPoster, r.setViewer).
		Summary("The poster of a video post").
		Describe("For a server that was sent the post: a redirect to the picture in storage.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.NoteRef{}).
		Unbound().
		SendsNothing(http.StatusFound, "the picture is at the address in Location").
		Refuses(http.StatusNotFound, "there is no such video of a post that went out")

	// A person's own choice to be on the fediverse.
	activitypub := api.Group("activitypub", r.authorize)
	activitypub.GET("/settings", r.handler.ReadSettings).
		Summary("Whether the viewer is on the fediverse, and could be").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.SettingsQuery{}).
		Unbound().
		Sends(http.StatusOK, handlers.Settings{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`person_unreadable`").
		Refuses(http.StatusForbidden, "somebody else's, and the viewer is not a site administrator").
		Refuses(http.StatusNotFound, "there is no such person")
	activitypub.PUT("/settings", r.handler.EditSettings).
		Summary("Go on the fediverse, or leave it").
		Describe("It is each person's own choice, off until they make it, and only for somebody whose profile is open to everybody.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.SettingsRequest{}).
		Sends(http.StatusOK, handlers.Settings{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`enabled_required`").
		Sends(http.StatusConflict, responses.Refusal{}, "why they cannot be: `site_private`, `site_unreachable` or `profile_not_public`")
	// Finding somebody on another server.
	activitypub.GET("/lookup", r.handler.Lookup).
		Summary("Find somebody on another server").
		Describe("They are kept here, so that they can be followed as anybody is. Thirty lookups in ten minutes.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.LookupRequest{}).
		Unbound().
		Sends(http.StatusOK, handlers.RemotePerson{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`lookup_unreadable`").
		Sends(http.StatusNotFound, responses.Refusal{}, "`lookup_not_found`, `lookup_is_here`, `lookup_not_a_person` or `lookup_refused`").
		Sends(http.StatusConflict, responses.Refusal{}, "the installation is not on the fediverse: `site_private` or `site_unreachable`").
		Sends(http.StatusTooManyRequests, responses.Refusal{}, "`too_many_lookups`").
		Sends(http.StatusBadGateway, responses.Refusal{}, "`lookup_failed`")
	// The pictures of a post from another server, copied here again after
	// they were removed for its age.
	activitypub.POST("/posts/:id/pictures", r.handler.RestorePictures).
		Summary("Copy a post's pictures here again").
		Describe("The pictures of a post from another server that has been left alone a while are removed from storage, and the post says so (`media_removed`). Asked here, they are read again from where they came from. For anybody signed in who may see the post.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.PostPicturesRequest{}).
		Unbound().
		Sends(http.StatusOK, responses.PicturesRestored{}).
		Sends(http.StatusNotFound, responses.Refusal{}, "`media_not_removed`: there is no such post the viewer may see, or its pictures are here").
		Sends(http.StatusGone, responses.Refusal{}, "`media_gone`").
		Sends(http.StatusTooManyRequests, responses.Refusal{}, "`too_many_requests`").
		Sends(http.StatusBadGateway, responses.Refusal{}, "`media_failed`")
	// The servers this installation refuses. Super administrators only:
	// the handlers check.
	activitypub.GET("/blocks", r.handler.BrowseBlocks).
		Summary("The servers refused").
		Viewer(apidoc.SuperAdmin).
		Takes(&handlers.BlocksBrowse{}).
		Unbound().
		Sends(http.StatusOK, responses.List[*federation.DomainBlock]{})
	activitypub.POST("/blocks", r.handler.AddBlock).
		Summary("Refuse a server, or change why").
		Describe("Nothing from it is taken, and nothing is sent to it.").
		Viewer(apidoc.SuperAdmin).
		Takes(&handlers.BlockRequest{}).
		Sends(http.StatusCreated, federation.DomainBlock{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`domain_required`: the request names no server; `domain_unreadable`: it is not a server's name; `domain_is_here`: it is this installation")
	activitypub.DELETE("/blocks/:domain", r.handler.DeleteBlock).
		Summary("Stop refusing a server").
		Viewer(apidoc.SuperAdmin).
		Takes(&handlers.DomainRef{}).
		Unbound().
		SendsNothing(http.StatusNoContent, "done").
		Refuses(http.StatusNotFound, "it is not refused")

	return doc
}
