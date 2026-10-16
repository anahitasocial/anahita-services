package main

import (
	"net/http"

	"photo/cmd/api/handlers"

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
// the descriptions: docs/api/photo-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("photo-service", "Photo posts",
		"Posts of up to four pictures, each with what it shows, and replies with pictures (photos.md).")
	api := doc.On(e)
	write := libs_constants.OAuthScopeMediaWrite
	media := requests.NewMediumRequest()
	reply := "`reply_follows_its_thread`: a reply has no setting of its own; its thread decides"
	files := "the images were refused; the answer says why"

	// photos — the read routes are public but still identify the caller
	// when a token is present, so the viewer drives access filtering.
	// Everything else requires a valid OAuth access token.
	photos := api.Group("photos")
	photos.GET("/:owner_id/", r.handler.Photo.Browse, r.setViewer).
		Summary("A profile's photo posts").
		Describe("Those the viewer may see.").
		Viewer(apidoc.Anyone).
		Takes(media.Browse).
		Sends(http.StatusOK, responses.List[*responses.MediumNodeItem]{})
	photos.GET("/:id", r.handler.Photo.Read, r.setViewer).
		Summary("One photo post").
		Viewer(apidoc.Anyone).
		Takes(media.Read).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not see it").
		Refuses(http.StatusNotFound, "there is no such post")
	photos.PATCH("/:id", r.handler.Photo.Edit, r.authorize).
		Summary("Change a photo post's words").
		Describe("Its title and words, both optional. Its pictures are changed with `PATCH /photos/{id}/files`.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.PhotoEditRequest{}).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such post")
	photos.POST("/:owner_id/", r.handler.Photo.Add, r.authorize).
		Summary("Post photos").
		Describe("As JSON naming the images uploaded for it (`POST /photos/{owner_id}/uploads`), in order: the way to post several. Or as a form carrying one image in `file`, as older clients do. A title and words are both optional: the pictures are the post.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(handlers.NewPhotoAddRequest()).
		Sends(http.StatusCreated, responses.MediumNode{}).
		Sends(http.StatusUnprocessableEntity, handlers.FilesRefusal{}).
		Refuses(http.StatusForbidden, "the viewer may not post here").
		Refuses(http.StatusNotFound, "there is no such profile")
	// A reply with pictures to the post :id: see photo_reply.go.
	photos.POST("/:id/replies", r.handler.Photo.AddReply, r.authorize).
		Summary("Reply with pictures").
		Describe("A photo post in the thread of the post `id`, owned by whoever writes it, as a reply with words alone is a note. Its pictures are uploaded first, for the person replying.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.PhotoReplyRequest{}).
		Sends(http.StatusCreated, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not reply here").
		Refuses(http.StatusNotFound, "there is no such post the viewer may see").
		Refuses(http.StatusGone, "it answers a reply that was removed")
	photos.DELETE("/:id", r.handler.Photo.Delete, r.authorize).
		Summary("Delete a photo post").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(media.Delete).
		SendsNothing(http.StatusOK, "it is gone").
		Refuses(http.StatusForbidden, "the viewer may not delete it").
		Refuses(http.StatusNotFound, "there is no such post")

	// The images of a post. Each is uploaded in a request of its own, and a
	// post is then made from the uploads; "files" changes the images of a
	// post that exists.
	photos.POST("/:owner_id/uploads", r.handler.Photo.Upload, r.authorize).
		Summary("Upload an image for a post").
		Describe("One image a request: a post with several is made from its uploads, so no request carries more than one. For whoever may post on the profile.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.PhotoUpload{}).
		Sends(http.StatusCreated, handlers.Uploaded{}).
		Sends(http.StatusBadRequest, handlers.FilesRefusal{}).
		Sends(http.StatusRequestEntityTooLarge, handlers.FilesRefusal{}).
		Sends(http.StatusUnsupportedMediaType, handlers.FilesRefusal{}).
		Refuses(http.StatusForbidden, "the viewer may not post here").
		Refuses(http.StatusNotFound, "there is no such profile")
	photos.PATCH("/:id/files", r.handler.Photo.EditFiles, r.authorize).
		Summary("Change a post's images").
		Describe("The whole list as it is to be: their order, what each shows, and which there are. One left out is removed; one named by an upload is added.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.PhotoFilesRequest{}).
		Sends(http.StatusOK, responses.MediumNode{}).
		Sends(http.StatusUnprocessableEntity, handlers.FilesRefusal{}).
		Refuses(http.StatusForbidden, "the viewer may not change it, or "+files).
		Refuses(http.StatusNotFound, "there is no such post, or "+files)

	// photo access
	photos.PATCH("/:id/access", r.handler.Photo.EditAccess, r.authorize).
		Summary("Change who may see it").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(media.EditAccess).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it, or it may not be shown to them").
		Refuses(http.StatusNotFound, "there is no such post").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, reply)

	// photo comment status
	photos.PATCH("/:id/comment-status", r.handler.Photo.EditCommentStatus, r.authorize).
		Summary("Open or close replies").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(media.EditCommentStatus).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such post").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, reply)
	photos.PATCH("/:id/reply-access", r.handler.Photo.EditReplyAccess, r.authorize).
		Summary("Change who may reply").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&requests.MediumEditReplyAccessRequest{}).
		Sends(http.StatusOK, responses.MediumNode{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`reply_access_invalid`, or not well formed").
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such post").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, reply)
	photos.PATCH("/:id/quote-policy", r.handler.Photo.EditQuotePolicy, r.authorize).
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
	photos.PATCH("/:id/download", r.handler.Photo.EditDownload, r.authorize).
		Summary("Change whether it may be downloaded").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.EditDownloadRequest{}).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such post")
	photos.GET("/:id/download", r.handler.Photo.Download, r.setViewer).
		Summary("Download an image").
		Describe("A redirect to the largest copy there is, named to be saved. For whoever may see the post, when it may be downloaded.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.PhotoDownloadRequest{}).
		Unbound().
		SendsNothing(http.StatusFound, "the file is at the address in Location").
		Refuses(http.StatusForbidden, "it may not be downloaded").
		Refuses(http.StatusNotFound, "there is no such post or image that the viewer may see").
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
	photos.PUT("/:id/pin", r.handler.Photo.Pin, r.authorize).With(pin("Pin it to the profile it is on"))
	photos.DELETE("/:id/pin", r.handler.Photo.Unpin, r.authorize).With(pin("Take its pin off"))

	return doc
}
