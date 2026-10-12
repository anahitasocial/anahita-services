package main

import (
	"video/cmd/api/handlers"

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

func (r *Routes) Init(e *echo.Echo) {
	// videos — the read routes are public but still identify the caller
	// when a token is present, so the viewer drives access filtering.
	// Everything else requires a valid OAuth access token.
	videos := e.Group("videos")

	// The uploads. Registered before the routes with an id in the same
	// place, which a word is never mistaken for.
	videos.GET("/uploads", r.handler.Video.UploadBrowse, r.authorize)
	videos.GET("/uploads/:id", r.handler.Video.UploadRead, r.authorize)
	videos.POST("/uploads/:id/parts", r.handler.Video.UploadParts, r.authorize)
	videos.POST("/uploads/:id/complete", r.handler.Video.UploadComplete, r.authorize)
	videos.DELETE("/uploads/:id", r.handler.Video.UploadCancel, r.authorize)
	videos.POST("/:owner_id/uploads", r.handler.Video.UploadStart, r.authorize)

	videos.GET("/:owner_id/", r.handler.Video.Browse, r.setViewer)
	videos.GET("/:id", r.handler.Video.Read, r.setViewer)
	videos.PATCH("/:id", r.handler.Video.Edit, r.authorize)
	videos.DELETE("/:id", r.handler.Video.Delete, r.authorize)

	// A video's files: each size of it, and its poster.
	videos.GET("/:id/files/:name", r.handler.Video.File, r.setViewer)

	videos.PATCH("/:id/access", r.handler.Video.EditAccess, r.authorize)
	videos.PATCH("/:id/comment-status", r.handler.Video.EditCommentStatus, r.authorize)
	videos.PATCH("/:id/reply-access", r.handler.Video.EditReplyAccess, r.authorize)
	videos.PATCH("/:id/quote-policy", r.handler.Video.EditQuotePolicy, r.authorize)
	// Whether it may be downloaded, and the download itself.
	videos.PATCH("/:id/download", r.handler.Video.EditDownload, r.authorize)
	videos.GET("/:id/download", r.handler.Video.Download, r.setViewer)
	videos.PUT("/:id/pin", r.handler.Video.Pin, r.authorize)
	videos.DELETE("/:id/pin", r.handler.Video.Unpin, r.authorize)
}
