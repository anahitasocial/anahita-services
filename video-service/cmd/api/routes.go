package main

import (
	"net/http"

	"video/cmd/api/handlers"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	"anahita-libs/requests"
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
// the descriptions: docs/api/video-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("video-service", "Video posts",
		"Posts of a video, a GIF or an animated PNG, sent straight to storage in parts and made ready by a worker in several sizes; and replies with one (videos.md). Optional: off until the installation turns video on.")
	api := doc.On(e)
	write := libs_constants.OAuthScopeMediaWrite
	media := requests.NewMediumRequest()
	reply := "`reply_follows_its_thread`: a reply has no setting of its own; its thread decides"
	mine := "there is no such upload of the viewer's"

	// videos — the read routes are public but still identify the caller
	// when a token is present, so the viewer drives access filtering.
	// Everything else requires a valid OAuth access token.
	videos := api.Group("videos")

	// The uploads. Registered before the routes with an id in the same
	// place, which a word is never mistaken for.
	videos.GET("/uploads", r.handler.Video.UploadBrowse, r.authorize).
		Summary("The viewer's uploads").
		Describe("Those being sent or waiting for the worker, and those that failed in the last day, the latest first.").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, responses.Unpaged[*handlers.UploadResponse]{})
	videos.GET("/uploads/:id", r.handler.Video.UploadRead, r.authorize).
		Summary("One upload").
		Describe("Asked again until it is `ready`, when `node_id` is the post, or `failed`, when `error` says why.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.UploadRef{}).
		Unbound().
		Sends(http.StatusOK, handlers.UploadResponse{}).
		Refuses(http.StatusNotFound, mine)
	videos.POST("/uploads/:id/parts", r.handler.Video.UploadParts, r.authorize).
		Summary("Where to send some parts").
		Describe("Signed addresses, good for an hour, each part sent to its own with PUT.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.UploadPartsRequest{}).
		Sends(http.StatusOK, responses.Unpaged[handlers.UploadPart]{}).
		Refuses(http.StatusNotFound, mine).
		Sends(http.StatusConflict, responses.Refusal{}, "`not_uploading`: it is not being sent any more")
	videos.POST("/uploads/:id/complete", r.handler.Video.UploadComplete, r.authorize).
		Summary("Say every part is sent").
		Describe("The worker then makes the post. Said twice, the answer is how it stands.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.UploadRef{}).
		Unbound().
		Sends(http.StatusOK, handlers.UploadResponse{}).
		Refuses(http.StatusNotFound, mine).
		Sends(http.StatusConflict, responses.Refusal{}, "`nothing_sent`").
		Sends(http.StatusRequestEntityTooLarge, responses.Refusal{}, "`too_large`: what was sent is larger than it may be")
	videos.DELETE("/uploads/:id", r.handler.Video.UploadCancel, r.authorize).
		Summary("Give an upload up").
		Describe("One being sent, waiting for the worker, or failed.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.UploadRef{}).
		Unbound().
		SendsNothing(http.StatusNoContent, "given up").
		Refuses(http.StatusNotFound, mine).
		Sends(http.StatusConflict, responses.Refusal{}, "`finished`: it is a post already")
	videos.POST("/:owner_id/uploads", r.handler.Video.UploadStart, r.authorize).
		Summary("Begin a video post").
		Describe("Everything about the post is settled here, as when any post is made, and the file is then sent straight to storage in parts (`POST /videos/uploads/{id}/parts`). With `parent_id`, a reply with a video, on its author's own profile.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.UploadStartRequest{}).
		Sends(http.StatusCreated, handlers.UploadResponse{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`language_invalid`, `reply_access_invalid` or `quote_policy_invalid`; or not well formed, without a body").
		Sends(http.StatusForbidden, responses.Refusal{}, "`uploads_off`: this installation takes no video; or the viewer may not post or reply here, without a body").
		Refuses(http.StatusNotFound, "there is no such profile, or nothing it answers that the viewer may see").
		Refuses(http.StatusGone, "it answers a reply that was removed").
		Sends(http.StatusRequestEntityTooLarge, handlers.UploadTooLarge{}).
		Sends(http.StatusUnsupportedMediaType, responses.Refusal{}, "`not_a_video`").
		Sends(http.StatusUnprocessableEntity, responses.AudienceRefusal{}).
		Sends(http.StatusTooManyRequests, responses.Refusal{}, "`too_many`: as many as may be begun in a day")

	videos.GET("/:owner_id/", r.handler.Video.Browse, r.setViewer).
		Summary("A profile's video posts").
		Describe("Those the viewer may see.").
		Viewer(apidoc.Anyone).
		Takes(media.Browse).
		Sends(http.StatusOK, responses.List[*responses.MediumNodeItem]{})
	videos.GET("/:id", r.handler.Video.Read, r.setViewer).
		Summary("One video post").
		Viewer(apidoc.Anyone).
		Takes(media.Read).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not see it").
		Refuses(http.StatusNotFound, "there is no such post")
	videos.PATCH("/:id", r.handler.Video.Edit, r.authorize).
		Summary("Change a video post's words").
		Describe("A video needs a title, and may have nothing more said of it.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.VideoEditRequest{}).
		Sends(http.StatusOK, responses.MediumNode{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`language_invalid`; or not well formed, without a body").
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such post")
	videos.DELETE("/:id", r.handler.Video.Delete, r.authorize).
		Summary("Delete a video post").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(media.Delete).
		SendsNothing(http.StatusOK, "it is gone").
		Refuses(http.StatusForbidden, "the viewer may not delete it").
		Refuses(http.StatusNotFound, "there is no such post")

	// A video's files: each size of it, and its poster.
	videos.GET("/:id/files/:name", r.handler.Video.File, r.setViewer).
		Summary("A file of a video post").
		Describe("A redirect to it in storage, at an address good for a short while. Who may see the post is asked each time; nothing in between keeps it.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.VideoFileRequest{}).
		Unbound().
		SendsNothing(http.StatusFound, "the file is at the address in Location").
		Refuses(http.StatusNotFound, "there is no such post or file that the viewer may see")

	videos.PATCH("/:id/access", r.handler.Video.EditAccess, r.authorize).
		Summary("Change who may see it").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(media.EditAccess).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such post").
		Sends(http.StatusUnprocessableEntity, responses.AudienceRefusal{}, "`access_not_allowed`, with the audiences it may have; or "+reply+", without them")
	videos.PATCH("/:id/comment-status", r.handler.Video.EditCommentStatus, r.authorize).
		Summary("Open or close replies").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(media.EditCommentStatus).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such post").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, reply)
	videos.PATCH("/:id/reply-access", r.handler.Video.EditReplyAccess, r.authorize).
		Summary("Change who may reply").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&requests.MediumEditReplyAccessRequest{}).
		Sends(http.StatusOK, responses.MediumNode{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`reply_access_invalid`, or not well formed").
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such post").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, reply)
	videos.PATCH("/:id/quote-policy", r.handler.Video.EditQuotePolicy, r.authorize).
		Summary("Change who may quote it").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&requests.MediumEditQuotePolicyRequest{}).
		Sends(http.StatusOK, responses.MediumNode{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`quote_policy_invalid`, or not well formed").
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such post").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, reply)
	// Whether it may be downloaded, and the download itself.
	videos.PATCH("/:id/download", r.handler.Video.EditDownload, r.authorize).
		Summary("Change whether it may be downloaded").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.EditDownloadRequest{}).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such post")
	videos.GET("/:id/download", r.handler.Video.Download, r.setViewer).
		Summary("Download it").
		Describe("A redirect to one size of it, named to be saved. For whoever may see the post, when it may be downloaded.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.VideoDownloadRequest{}).
		Unbound().
		SendsNothing(http.StatusFound, "the file is at the address in Location").
		Refuses(http.StatusForbidden, "it may not be downloaded").
		Refuses(http.StatusNotFound, "there is no such post or size that the viewer may see").
		Refuses(http.StatusTooManyRequests, "too many downloads lately")
	pin := func(what string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.
				Summary(what).
				Describe("A profile has one pin: pinning another post moves it. For the profile and its administrators, and site administrators (permissions.md). Answers with the post.").
				Viewer(apidoc.SignedIn).
				Scope(write).
				Takes(media.Read).
				Sends(http.StatusOK, responses.MediumNode{}).
				Refuses(http.StatusForbidden, "the viewer may not pin it").
				Refuses(http.StatusNotFound, "there is no such post").
				Sends(http.StatusUnprocessableEntity, responses.Refusal{}, "`reply_cannot_be_pinned`")
		}
	}
	videos.PUT("/:id/pin", r.handler.Video.Pin, r.authorize).With(pin("Pin it to the profile it is on"))
	videos.DELETE("/:id/pin", r.handler.Video.Unpin, r.authorize).With(pin("Take its pin off"))

	return doc
}
