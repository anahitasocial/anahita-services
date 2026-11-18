package main

import (
	"anahita-libs/apidoc"
)

// --- Articles ---
// Articles, which may begin as a draft and be published later. Reading
// them is public, and the caller is known when a token is sent
// (r.setViewer); everything else needs a token (r.authorize).
func (r *Routes) articlesRoutes(api *apidoc.Router, d postDocs) {
	media := d.media

	articles := api.Group("articles")

	articles.GET("/:owner_id/", r.handler.Article.Browse, r.setViewer).With(d.browse("article"))

	articles.GET("/:id", r.handler.Article.Read, r.setViewer).With(d.read("article"))

	articles.PATCH("/:id", r.handler.Article.Edit, r.authorize).With(d.edit("article", media.Edit))

	articles.POST("/:owner_id/", r.handler.Article.Add, r.authorize).
		With(d.add("article", media.Add))

	articles.DELETE("/:id", r.handler.Article.Delete, r.authorize).With(d.remove("article"))

	articles.POST("/:id/publish", r.handler.Article.Publish, r.authorize).With(d.publish)

	// --- Settings ---
	// An article's settings, as any post's: who may see it, reply to it and
	// quote it, its content warning, and its pin. Each needs a token
	// (r.authorize).
	articles.PATCH("/:id/access", r.handler.Article.EditAccess, r.authorize).With(d.access)

	articles.PATCH("/:id/comment-status", r.handler.Article.EditCommentStatus, r.authorize).
		With(d.commentStatus)

	articles.PATCH("/:id/reply-access", r.handler.Article.EditReplyAccess, r.authorize).
		With(d.replyAccess)

	articles.PATCH("/:id/quote-policy", r.handler.Article.EditQuotePolicy, r.authorize).
		With(d.quotePolicy)

	articles.PATCH("/:id/warning", r.handler.Article.EditWarning, r.authorize).With(d.warning)

	articles.PUT("/:id/pin", r.handler.Article.Pin, r.authorize).
		With(d.pin("Pin it to the profile it is on"))

	articles.DELETE("/:id/pin", r.handler.Article.Unpin, r.authorize).
		With(d.pin("Take its pin off"))
}

// --- Topics ---
// Topics, described as articles are: they may begin as a draft too.
// Reading them is public, and the caller is known when a token is sent
// (r.setViewer); everything else needs a token (r.authorize).
func (r *Routes) topicsRoutes(api *apidoc.Router, d postDocs) {
	media := d.media

	topics := api.Group("topics")

	topics.GET("/:owner_id/", r.handler.Topic.Browse, r.setViewer).With(d.browse("topic"))

	topics.GET("/:id", r.handler.Topic.Read, r.setViewer).With(d.read("topic"))

	topics.PATCH("/:id", r.handler.Topic.Edit, r.authorize).With(d.edit("topic", media.Edit))

	topics.POST("/:owner_id/", r.handler.Topic.Add, r.authorize).
		With(d.add("topic", media.Add))

	topics.DELETE("/:id", r.handler.Topic.Delete, r.authorize).With(d.remove("topic"))

	topics.POST("/:id/publish", r.handler.Topic.Publish, r.authorize).With(d.publish)

	// --- Settings ---
	// A topic's settings, as any post's: who may see it, reply to it and
	// quote it, its content warning, and its pin. Each needs a token
	// (r.authorize).
	topics.PATCH("/:id/access", r.handler.Topic.EditAccess, r.authorize).With(d.access)

	topics.PATCH("/:id/comment-status", r.handler.Topic.EditCommentStatus, r.authorize).
		With(d.commentStatus)

	topics.PATCH("/:id/reply-access", r.handler.Topic.EditReplyAccess, r.authorize).
		With(d.replyAccess)

	topics.PATCH("/:id/quote-policy", r.handler.Topic.EditQuotePolicy, r.authorize).
		With(d.quotePolicy)

	topics.PATCH("/:id/warning", r.handler.Topic.EditWarning, r.authorize).With(d.warning)

	topics.PUT("/:id/pin", r.handler.Topic.Pin, r.authorize).
		With(d.pin("Pin it to the profile it is on"))

	topics.DELETE("/:id/pin", r.handler.Topic.Unpin, r.authorize).
		With(d.pin("Take its pin off"))
}
