package main

import (
	"encoding/json"
	"net/http"

	"activitypub/ap"
	"activitypub/cmd/api/handlers"

	"anahita-libs/apidoc"
)

// --- What other servers ask ---
// Finding somebody here, sending activities here, and reading what went
// out. Public: no token is needed, since a server has none, and who is
// answered for is decided in each handler: it is only ever somebody who
// chose to be on the fediverse. r.setViewer is on each all the same,
// because it is what answers for a site that is for members only.
func (r *Routes) serverRoutes(api *apidoc.Router) {
	// The media type of what is sent and taken in ActivityStreams.
	activity := "application/activity+json"

	// The opening of the description of each route that answers in
	// ActivityStreams.
	server := "For other servers, in ActivityStreams " +
		"(https://www.w3.org/TR/activitystreams-vocabulary/). "

	// The refusal of somebody who is not here, or not on the fediverse.
	gone := "nobody here by that name is on the fediverse"

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

	// --- Inboxes ---
	// Where other servers send what somebody did: one for the whole
	// installation, and one for each person. They are the same door, so
	// inbox describes both.
	inbox := func(route *apidoc.Route) {
		route.
			Summary("Take an activity from another server").
			Describe(server+"Signed with HTTP Signatures; what is not signed by whoever is said "+
				"to have done it is not believed. Six hundred a minute from one address. Taken, "+
				"whether or not anything is done with it.").
			Viewer(apidoc.Anyone).
			Media(activity).
			TakesBody(json.RawMessage{}).
			SendsNothing(http.StatusAccepted, "taken").
			Refuses(http.StatusUnauthorized, "not signed, or the signature does not check, or not "+
				"by whoever is said to have done it").
			Refuses(http.StatusForbidden, "the server it came from is refused").
			Refuses(http.StatusNotFound, gone).
			Refuses(http.StatusRequestEntityTooLarge, "too large").
			Refuses(http.StatusTooManyRequests, "too many from one address").
			Refuses(http.StatusServiceUnavailable, "it could not be kept now; send it again")
	}
	api.POST("/inbox", r.handler.Inbox, r.setViewer).With(inbox)

	// --- Profiles and posts, as other servers read them ---
	// Under /ap, as each actor's and each post's address says. A browser
	// asking for a profile or a post is sent to the web app instead.
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
		Describe(server+"A `Person`, or an `Organization` for a Channel, with their public key.").
		Viewer(apidoc.Anyone).
		Media(activity).
		Takes(&handlers.ActorRef{}).
		Unbound().
		Sends(http.StatusOK, ap.Actor{}).
		SendsNothing(http.StatusFound, "a browser, asking for text/html, is sent to their profile "+
			"in the web app, the actor's `url`").
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
		SendsNothing(http.StatusFound, "a browser, asking for text/html, is sent to the post's "+
			"page in the web app, the note's `url`").
		Refuses(http.StatusNotFound, "there is no such post that went out")

	// A stamp: a quote of a post here approved (roadmap row 156, FEP-044f).
	ap_.GET("/quotes/:uuid", r.handler.QuoteStamp, r.setViewer).
		Summary("A quote's stamp").
		Describe(server+"A `QuoteAuthorization`: the author of a post here let another post quote "+
			"it. Given to a post on another server that asked with a `QuoteRequest`, or made for "+
			"a post here that quotes another here, and there only while the quote is.").
		Viewer(apidoc.Anyone).
		Media(activity).
		Takes(&handlers.NoteRef{}).
		Unbound().
		Sends(http.StatusOK, ap.QuoteAuthorization{}).
		Refuses(http.StatusNotFound, "there is no such stamp, or the quote it approved is gone")

	// A post's files, each a redirect to storage, for a server that was
	// sent the post: a photo post's pictures, a video post's video and its
	// poster.
	ap_.GET("/notes/:uuid/pictures/:position", r.handler.Picture, r.setViewer).
		Summary("A picture of a post").
		Describe("For a server that was sent the post: a redirect to the picture in storage.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.NotePictureRef{}).
		Unbound().
		SendsNothing(http.StatusFound, "the picture is at the address in Location").
		Refuses(http.StatusNotFound, "there is no such picture of a post that went out")

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
}
