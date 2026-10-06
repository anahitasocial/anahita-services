package main

import (
	an_mw "anahita-libs/middleware"
	"text/cmd/api/handlers"

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
func (r *Routes) Init(e *echo.Echo) {
	// articles
	articles := e.Group("articles")
	articles.GET("/:owner_id/", r.handler.Article.Browse, r.setViewer)
	articles.GET("/:id", r.handler.Article.Read, r.setViewer)
	articles.PATCH("/:id", r.handler.Article.Edit, r.authorize)
	articles.POST("/:owner_id/", r.handler.Article.Add, r.authorize)
	articles.DELETE("/:id", r.handler.Article.Delete, r.authorize)

	// article access
	articles.PATCH("/:id/access", r.handler.Article.EditAccess, r.authorize)

	// article comment status
	articles.PATCH("/:id/comment-status", r.handler.Article.EditCommentStatus, r.authorize)
	articles.PATCH("/:id/reply-access", r.handler.Article.EditReplyAccess, r.authorize)
	articles.PATCH("/:id/quote-policy", r.handler.Article.EditQuotePolicy, r.authorize)
	articles.PUT("/:id/pin", r.handler.Article.Pin, r.authorize)
	articles.DELETE("/:id/pin", r.handler.Article.Unpin, r.authorize)

	// notes
	notes := e.Group("notes")
	notes.GET("/:owner_id/", r.handler.Note.Browse, r.setViewer)
	notes.GET("/:id", r.handler.Note.Read, r.setViewer)
	notes.PATCH("/:id", r.handler.Note.Edit, r.authorize)
	notes.POST("/:owner_id/", r.handler.Note.Add, r.authorize)
	notes.DELETE("/:id", r.handler.Note.Delete, r.authorize)

	// Replies. A reply is a note that answers a post or another reply, so
	// it is read, edited and removed at /notes/:id like any note. These are
	// what only a thread has. The post answered can be of any kind.
	// Read by members only where visitors are given a preview: a preview
	// shows a post and how much was said under it, not what was said.
	notes.GET("/:id/thread", r.handler.Note.Thread, r.setViewer, an_mw.MembersInPreview())
	notes.POST("/:id/replies", r.handler.Note.AddReply, r.authorize)
	notes.PATCH("/:id/hidden", r.handler.Note.HideReply, r.authorize)

	// Taking a quoted post out of a note that quotes it, for that post's
	// author.
	notes.DELETE("/:id/quote", r.handler.Note.DetachQuote, r.authorize)

	// note access
	notes.PATCH("/:id/access", r.handler.Note.EditAccess, r.authorize)

	// note comment status
	notes.PATCH("/:id/comment-status", r.handler.Note.EditCommentStatus, r.authorize)
	notes.PATCH("/:id/reply-access", r.handler.Note.EditReplyAccess, r.authorize)
	notes.PATCH("/:id/quote-policy", r.handler.Note.EditQuotePolicy, r.authorize)
	notes.PUT("/:id/pin", r.handler.Note.Pin, r.authorize)
	notes.DELETE("/:id/pin", r.handler.Note.Unpin, r.authorize)

	// topics
	topics := e.Group("topics")
	topics.GET("/:owner_id/", r.handler.Topic.Browse, r.setViewer)
	topics.GET("/:id", r.handler.Topic.Read, r.setViewer)
	topics.PATCH("/:id", r.handler.Topic.Edit, r.authorize)
	topics.POST("/:owner_id/", r.handler.Topic.Add, r.authorize)
	topics.DELETE("/:id", r.handler.Topic.Delete, r.authorize)

	// topic access
	topics.PATCH("/:id/access", r.handler.Topic.EditAccess, r.authorize)

	// topic comment status
	topics.PATCH("/:id/comment-status", r.handler.Topic.EditCommentStatus, r.authorize)
	topics.PATCH("/:id/reply-access", r.handler.Topic.EditReplyAccess, r.authorize)
	topics.PATCH("/:id/quote-policy", r.handler.Topic.EditQuotePolicy, r.authorize)
	topics.PUT("/:id/pin", r.handler.Topic.Pin, r.authorize)
	topics.DELETE("/:id/pin", r.handler.Topic.Unpin, r.authorize)
}
