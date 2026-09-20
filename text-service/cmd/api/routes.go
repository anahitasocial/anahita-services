package main

import (
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
	articles.GET("/", r.handler.Article.Browse, r.setViewer)
	articles.GET("/:owner_id/", r.handler.Article.Browse, r.setViewer)
	articles.GET("/:id", r.handler.Article.Read, r.setViewer)
	articles.PATCH("/:id", r.handler.Article.Edit, r.authorize)
	articles.POST("/:owner_id/", r.handler.Article.Add, r.authorize)
	articles.DELETE("/:id", r.handler.Article.Delete, r.authorize)

	// article access
	articles.PATCH("/:id/access", r.handler.Article.EditAccess, r.authorize)

	// article comment status
	articles.PATCH("/:id/comment-status", r.handler.Article.EditCommentStatus, r.authorize)

	// notes
	notes := e.Group("notes")
	notes.GET("/", r.handler.Note.Browse, r.setViewer)
	notes.GET("/:owner_id/", r.handler.Note.Browse, r.setViewer)
	notes.GET("/:id", r.handler.Note.Read, r.setViewer)
	notes.PATCH("/:id", r.handler.Note.Edit, r.authorize)
	notes.POST("/:owner_id/", r.handler.Note.Add, r.authorize)
	notes.DELETE("/:id", r.handler.Note.Delete, r.authorize)

	// note access
	notes.PATCH("/:id/access", r.handler.Note.EditAccess, r.authorize)

	// note comment status
	notes.PATCH("/:id/comment-status", r.handler.Note.EditCommentStatus, r.authorize)

	// topics
	topics := e.Group("topics")
	topics.GET("/", r.handler.Topic.Browse, r.setViewer)
	topics.GET("/:owner_id/", r.handler.Topic.Browse, r.setViewer)
	topics.GET("/:id", r.handler.Topic.Read, r.setViewer)
	topics.PATCH("/:id", r.handler.Topic.Edit, r.authorize)
	topics.POST("/:owner_id/", r.handler.Topic.Add, r.authorize)
	topics.DELETE("/:id", r.handler.Topic.Delete, r.authorize)

	// topic access
	topics.PATCH("/:id/access", r.handler.Topic.EditAccess, r.authorize)

	// topic comment status
	topics.PATCH("/:id/comment-status", r.handler.Topic.EditCommentStatus, r.authorize)
}
