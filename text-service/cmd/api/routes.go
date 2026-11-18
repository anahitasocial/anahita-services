package main

import (
	"net/http"

	"text/cmd/api/handlers"

	"anahita-libs/apidoc"
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
// the descriptions: docs/api/text-service.json is made of them. For each
// kind of post the reads are public, and the caller is known when a token
// is sent (r.setViewer); the writes need a token (r.authorize).
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("text-service", "Notes, articles and topics",
		"Posts of words: notes (short, and every reply), articles and topics; threads of replies; "+
			"quotes; and the posts a person saves.")
	api := doc.On(e)

	// What is described the same way for each kind of post.
	d := newPostDocs()

	// The posts a person saves, of any kind.
	r.bookmarksRoutes(api, d)

	// Articles, which may begin as a draft.
	r.articlesRoutes(api, d)

	// Notes, and the replies, threads and quotes that are notes too.
	r.notesRoutes(api, d)

	// Topics, which are described as articles are.
	r.topicsRoutes(api, d)

	return doc
}

// --- Saved posts ---
// Each person's own, of posts of any kind, so every route needs a token
// (r.authorize).
func (r *Routes) bookmarksRoutes(api *apidoc.Router, d postDocs) {
	bookmarks := api.Group("bookmarks", r.authorize)

	bookmarks.GET("/", r.handler.Note.Saved).
		Summary("The viewer's saved posts").
		Describe("Of any kind, the most lately saved first.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.SavedBrowseRequest{Limit: 20}).
		Sends(http.StatusOK, responses.List[*responses.MediumNodeItem]{})

	bookmarks.PUT("/:id", r.handler.Note.Save).With(d.save("Save a post"))

	bookmarks.DELETE("/:id", r.handler.Note.Unsave).With(d.save("Unsave a post"))
}
