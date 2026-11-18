package main

import (
	"net/http"

	"activitypub/cmd/api/handlers"

	"anahita-libs/apidoc"
	"anahita-libs/responses"
)

// --- Somebody on another server ---
// Finding them, reading them again when what was kept of them is old
// (roadmap row 130), and copying their posts' pictures here again after
// they were removed for their age. Every route needs a token
// (r.authorize, on the group).
func (r *Routes) remoteRoutes(activitypub *apidoc.Router) {
	activitypub.GET("/lookup", r.handler.Lookup).
		Summary("Find somebody on another server").
		Describe("They are kept here, so that they can be followed as anybody is. Thirty lookups "+
			"in ten minutes.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.LookupRequest{}).
		Unbound().
		Sends(http.StatusOK, handlers.RemotePerson{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`lookup_unreadable`").
		Sends(http.StatusNotFound, responses.Refusal{}, "`lookup_not_found`, `lookup_is_here`, "+
			"`lookup_not_a_person` or `lookup_refused`").
		Sends(http.StatusConflict, responses.Refusal{}, "the installation is not on the "+
			"fediverse: `site_private` or `site_unreachable`").
		Sends(http.StatusTooManyRequests, responses.Refusal{}, "`too_many_lookups`").
		Sends(http.StatusBadGateway, responses.Refusal{}, "`lookup_failed`")

	activitypub.POST("/people/:id/refresh", r.handler.RefreshPerson).
		Summary("Read somebody on another server again").
		Describe("When what was kept of them is more than an hour old: whether they approve their "+
			"followers, their name, their words, their pictures. Asked by the web app when their "+
			"profile is opened; `refreshed` says whether to read the profile again. A failure "+
			"there leaves what was kept, and says nothing.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.RefreshPersonRequest{}).
		Unbound().
		Sends(http.StatusOK, handlers.Refreshed{}).
		Sends(http.StatusNotFound, responses.Refusal{}, "`profile_not_kept`: nobody from another "+
			"server is kept under that id").
		Sends(http.StatusConflict, responses.Refusal{}, "the installation is not on the "+
			"fediverse: `site_private` or `site_unreachable`")

	activitypub.POST("/posts/:id/pictures", r.handler.RestorePictures).
		Summary("Copy a post's pictures here again").
		Describe("The pictures of a post from another server that has been left alone a while are "+
			"removed from storage, and the post says so (`media_removed`). Asked here, they are "+
			"read again from where they came from. For anybody signed in who may see the post.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.PostPicturesRequest{}).
		Unbound().
		Sends(http.StatusOK, responses.PicturesRestored{}).
		Sends(http.StatusNotFound, responses.Refusal{}, "`media_not_removed`: there is no such "+
			"post the viewer may see, or its pictures are here").
		Sends(http.StatusGone, responses.Refusal{}, "`media_gone`").
		Sends(http.StatusTooManyRequests, responses.Refusal{}, "`too_many_requests`").
		Sends(http.StatusBadGateway, responses.Refusal{}, "`media_failed`")
}
