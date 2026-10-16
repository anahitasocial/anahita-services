package main

import (
	"net/http"

	"text/cmd/api/handlers"
	"text/requests"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	an_mw "anahita-libs/middleware"
	libs_requests "anahita-libs/requests"
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

// Init registers the routes. In each group the read routes are public
// but still identify the caller when a token is present, so the viewer
// drives access filtering; the writes require a valid access token.
// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/text-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("text-service", "Notes, articles and topics",
		"Posts of words: notes (short, and every reply), articles and topics; threads of replies; quotes; and the posts a person saves.")
	api := doc.On(e)
	write := libs_constants.OAuthScopeMediaWrite
	media := libs_requests.NewMediumRequest()
	notesReq := requests.NewNoteRequest()
	reply := "`reply_follows_its_thread`: a reply has no setting of its own; its thread decides"
	invalid := "`language_invalid`, `reply_access_invalid` or `quote_policy_invalid`; or not well formed, without a body"

	// What is the same for each kind of post.
	browse := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("A profile's "+kind+"s").
				Describe("Those the viewer may see.").
				Viewer(apidoc.Anyone).
				Takes(media.Browse).
				Sends(http.StatusOK, responses.List[*responses.MediumNodeItem]{}).
				Refuses(http.StatusForbidden, "the viewer may not see the profile")
		}
	}
	read := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("One "+kind).
				Describe("A reply is read by whoever may read the post at the top of its thread.").
				Viewer(apidoc.Anyone).
				Takes(media.Read).
				Sends(http.StatusOK, responses.MediumNode{}).
				Refuses(http.StatusForbidden, "the viewer may not see it").
				Refuses(http.StatusNotFound, "there is no such post").
				Refuses(http.StatusGone, "a reply that was removed")
		}
	}
	edit := func(kind string, takes any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Change a "+kind+"'s words").
				Viewer(apidoc.SignedIn).
				Scope(write).
				Takes(takes).
				Sends(http.StatusOK, responses.MediumNode{}).
				Sends(http.StatusBadRequest, responses.Refusal{}, "`language_invalid`; or not well formed, without a body").
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such post")
		}
	}
	add := func(kind string, takes any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Post a "+kind).
				Viewer(apidoc.SignedIn).
				Scope(write).
				Takes(takes).
				Sends(http.StatusCreated, responses.MediumNode{}).
				Sends(http.StatusBadRequest, responses.Refusal{}, invalid).
				Refuses(http.StatusForbidden, "the viewer may not post here").
				Refuses(http.StatusNotFound, "there is no such profile").
				Sends(http.StatusUnprocessableEntity, responses.AudienceRefusal{}, "`access_not_allowed`, with the audiences it may have")
		}
	}
	remove := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Delete a "+kind).
				Describe("A reply that others have answered is emptied and kept, so the thread holds together.").
				Viewer(apidoc.SignedIn).
				Scope(write).
				Takes(media.Delete).
				SendsNothing(http.StatusOK, "it is gone").
				Refuses(http.StatusForbidden, "the viewer may not delete it").
				Refuses(http.StatusNotFound, "there is no such post")
		}
	}
	setting := func(summary string, takes any, bad string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary(summary).
				Viewer(apidoc.SignedIn).
				Scope(write).
				Takes(takes).
				Sends(http.StatusOK, responses.MediumNode{}).
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such post").
				Sends(http.StatusUnprocessableEntity, responses.Refusal{}, reply)
			if bad != "" {
				route.Sends(http.StatusBadRequest, responses.Refusal{}, bad)
			}
		}
	}
	access := func(route *apidoc.Route) {
		route.Summary("Change who may see it").
			Viewer(apidoc.SignedIn).
			Scope(write).
			Takes(media.EditAccess).
			Sends(http.StatusOK, responses.MediumNode{}).
			Refuses(http.StatusForbidden, "the viewer may not change it").
			Refuses(http.StatusNotFound, "there is no such post").
			Sends(http.StatusUnprocessableEntity, responses.AudienceRefusal{}, "`access_not_allowed`, with the audiences it may have; or "+reply+", without them")
	}
	pin := func(what string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary(what).
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
	commentStatus := setting("Open or close replies", media.EditCommentStatus, "")
	replyAccess := setting("Change who may reply", &libs_requests.MediumEditReplyAccessRequest{}, "`reply_access_invalid`, or not well formed")
	quotePolicy := setting("Change who may quote it", &libs_requests.MediumEditQuotePolicyRequest{}, "`quote_policy_invalid`, or not well formed")
	save := func(what string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary(what).
				Describe("A post of any kind. Saving is the viewer's own: nobody is told.").
				Viewer(apidoc.SignedIn).
				Takes(media.Read).
				Sends(http.StatusOK, handlers.SavedAnswer{}).
				Refuses(http.StatusNotFound, "there is no such post that the viewer may see")
		}
	}

	// saved posts — each person's own, so every route needs a token. For
	// posts of any kind.
	bookmarks := api.Group("bookmarks", r.authorize)
	bookmarks.GET("/", r.handler.Note.Saved).
		Summary("The viewer's saved posts").
		Describe("Of any kind, the most lately saved first.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.SavedBrowseRequest{Limit: 20}).
		Sends(http.StatusOK, responses.List[*responses.MediumNodeItem]{})
	bookmarks.PUT("/:id", r.handler.Note.Save).With(save("Save a post"))
	bookmarks.DELETE("/:id", r.handler.Note.Unsave).With(save("Unsave a post"))

	// articles
	articles := api.Group("articles")
	articles.GET("/:owner_id/", r.handler.Article.Browse, r.setViewer).With(browse("article"))
	articles.GET("/:id", r.handler.Article.Read, r.setViewer).With(read("article"))
	articles.PATCH("/:id", r.handler.Article.Edit, r.authorize).With(edit("article", media.Edit))
	articles.POST("/:owner_id/", r.handler.Article.Add, r.authorize).With(add("article", media.Add))
	articles.DELETE("/:id", r.handler.Article.Delete, r.authorize).With(remove("article"))

	// article access
	articles.PATCH("/:id/access", r.handler.Article.EditAccess, r.authorize).With(access)

	// article comment status
	articles.PATCH("/:id/comment-status", r.handler.Article.EditCommentStatus, r.authorize).With(commentStatus)
	articles.PATCH("/:id/reply-access", r.handler.Article.EditReplyAccess, r.authorize).With(replyAccess)
	articles.PATCH("/:id/quote-policy", r.handler.Article.EditQuotePolicy, r.authorize).With(quotePolicy)
	articles.PUT("/:id/pin", r.handler.Article.Pin, r.authorize).With(pin("Pin it to the profile it is on"))
	articles.DELETE("/:id/pin", r.handler.Article.Unpin, r.authorize).With(pin("Take its pin off"))

	// notes
	notes := api.Group("notes")
	notes.GET("/:owner_id/", r.handler.Note.Browse, r.setViewer).With(browse("note"))
	notes.GET("/:id", r.handler.Note.Read, r.setViewer).With(read("note"))
	notes.PATCH("/:id", r.handler.Note.Edit, r.authorize).
		With(edit("note", notesReq.Edit)).
		Describe("A reply is changed by its author alone.").
		Refuses(http.StatusGone, "a reply that was removed")
	notes.POST("/:owner_id/", r.handler.Note.Add, r.authorize).
		With(add("note", notesReq.Add)).
		Describe("With `quote_id`, a quote: the post is carried under the note's words, as its author's quote policy lets. With `link_preview_url`, the card of a link the writer was shown; what the card says is what this installation read, never what is sent.").
		Sends(http.StatusForbidden, responses.Refusal{}, "`quote_not_allowed`, with `reason`: the post may not be quoted by the viewer")

	// Replies. A reply is a note that answers a post or another reply, so
	// it is read, edited and removed at /notes/:id like any note. These are
	// what only a thread has. The post answered can be of any kind.
	// Read by members only where visitors are given a preview: a preview
	// shows a post and how much was said under it, not what was said.
	notes.GET("/:id/thread", r.handler.Note.Thread, r.setViewer, an_mw.MembersInPreview()).
		Summary("The replies under a post").
		Describe("Of a post of any kind, as the viewer may see them, with whether they may reply. On an installation in preview, only for somebody signed in.").
		Viewer(apidoc.Anyone).
		Takes(media.Read).
		Sends(http.StatusOK, handlers.Thread{}).
		Refuses(http.StatusUnauthorized, "the installation is in preview and the viewer is not signed in").
		Refuses(http.StatusForbidden, "the viewer may not see the post").
		Refuses(http.StatusNotFound, "there is no such post").
		Sends(http.StatusConflict, handlers.NotARoot{}, "it is a reply: its thread is the one of `root_id`")
	notes.POST("/:id/replies", r.handler.Note.AddReply, r.authorize).
		Summary("Reply").
		Describe("A note in the thread of the post or the reply `id`, owned by whoever writes it, seen by whoever may see the post at the top of the thread. Who may reply is that post's to say.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.ReplyAddRequest{}).
		Sends(http.StatusCreated, responses.MediumNode{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`language_invalid`; or not well formed, without a body").
		Refuses(http.StatusForbidden, "the viewer may not reply here").
		Refuses(http.StatusNotFound, "there is no such post the viewer may see").
		Refuses(http.StatusGone, "it answers a reply that was removed")
	notes.PATCH("/:id/hidden", r.handler.Note.HideReply, r.authorize).
		Summary("Hide a reply, or show it again").
		Describe("From everybody but its writer and whoever looks after the thread, with everything said under it. It is not deleted, and its writer is not told. A reply from Bluesky is hidden there too, when the post went there (federation.md).").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.ReplyHideRequest{}).
		Sends(http.StatusOK, handlers.HiddenAnswer{}).
		Refuses(http.StatusForbidden, "the viewer does not look after the thread").
		Refuses(http.StatusNotFound, "there is no such reply")

	// Taking a quoted post out of a note that quotes it, for that post's
	// author.
	notes.DELETE("/:id/quote", r.handler.Note.DetachQuote, r.authorize).
		Summary("Take one's post out of a quote").
		Describe("For the author of the quoted post. The note stays, saying the post was removed by its author. It cannot be undone.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(media.Read).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer is not the quoted post's author").
		Refuses(http.StatusNotFound, "there is no such note the viewer may see").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, "`not_a_quote`")
	// Who quoted a post, of any kind. Not part of a preview, like a thread.
	notes.GET("/:id/quotes", r.handler.Note.Quotes, r.setViewer, an_mw.MembersInPreview()).
		Summary("The notes that quote a post").
		Describe("Those the viewer may see.").
		Viewer(apidoc.Anyone).
		Takes(media.Read).
		Sends(http.StatusOK, responses.Unpaged[*responses.MediumNode]{}).
		Refuses(http.StatusUnauthorized, "the installation is in preview and the viewer is not signed in").
		Refuses(http.StatusNotFound, "there is no such post that the viewer may see")

	// note access
	notes.PATCH("/:id/access", r.handler.Note.EditAccess, r.authorize).With(access)

	// note comment status
	notes.PATCH("/:id/comment-status", r.handler.Note.EditCommentStatus, r.authorize).With(commentStatus)
	notes.PATCH("/:id/reply-access", r.handler.Note.EditReplyAccess, r.authorize).With(replyAccess)
	notes.PATCH("/:id/quote-policy", r.handler.Note.EditQuotePolicy, r.authorize).With(quotePolicy)
	notes.PUT("/:id/pin", r.handler.Note.Pin, r.authorize).With(pin("Pin it to the profile it is on"))
	notes.DELETE("/:id/pin", r.handler.Note.Unpin, r.authorize).With(pin("Take its pin off"))

	// topics
	topics := api.Group("topics")
	topics.GET("/:owner_id/", r.handler.Topic.Browse, r.setViewer).With(browse("topic"))
	topics.GET("/:id", r.handler.Topic.Read, r.setViewer).With(read("topic"))
	topics.PATCH("/:id", r.handler.Topic.Edit, r.authorize).With(edit("topic", media.Edit))
	topics.POST("/:owner_id/", r.handler.Topic.Add, r.authorize).With(add("topic", media.Add))
	topics.DELETE("/:id", r.handler.Topic.Delete, r.authorize).With(remove("topic"))

	// topic access
	topics.PATCH("/:id/access", r.handler.Topic.EditAccess, r.authorize).With(access)

	// topic comment status
	topics.PATCH("/:id/comment-status", r.handler.Topic.EditCommentStatus, r.authorize).With(commentStatus)
	topics.PATCH("/:id/reply-access", r.handler.Topic.EditReplyAccess, r.authorize).With(replyAccess)
	topics.PATCH("/:id/quote-policy", r.handler.Topic.EditQuotePolicy, r.authorize).With(quotePolicy)
	topics.PUT("/:id/pin", r.handler.Topic.Pin, r.authorize).With(pin("Pin it to the profile it is on"))
	topics.DELETE("/:id/pin", r.handler.Topic.Unpin, r.authorize).With(pin("Take its pin off"))

	return doc
}
