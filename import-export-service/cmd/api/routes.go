package main

import (
	"net/http"

	"importexport/cmd/api/handlers"

	"anahita-libs/apidoc"
	"anahita-libs/constants"
	libs_responses "anahita-libs/responses"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Routes struct {
	handler   *handlers.ExportsHandler
	imports   *handlers.ImportsHandler
	authorize echo.MiddlewareFunc
}

func NewRoutes(env *handlers.Env, authorize echo.MiddlewareFunc) *Routes {
	return &Routes{
		handler:   handlers.NewExportsHandler(env),
		imports:   handlers.NewImportsHandler(env),
		authorize: authorize,
	}
}

// UploadLimit is how large an import's request may be: a file of
// importfile.MaxBytes and the form around it. Every other route keeps the
// limit every service has.
const UploadLimit = "21M"

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/import-export-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	h := r.handler
	doc := apidoc.NewService("import-export-service", "Data exports and imports",
		"A person's copy of their own data: asking for one, seeing how it is getting on, fetching it and giving it up (data-export.md); and bringing their lists in from a file (moving.md). Optional: an installation may not run it.")
	api := doc.On(e)

	// Every route is the viewer's own: there is nothing here for a visitor.
	exports := api.Group("exports", r.authorize)
	exports.GET("", h.Browse).
		Summary("The viewer's exports").
		Describe("And how often they may ask, how long an archive is kept, and when they may next ask.").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.ExportList{})
	exports.POST("", h.Add).
		Summary("Ask for a copy of the viewer's data").
		Describe("Everything they wrote, as a zip archive made in the background: `GET /exports/:id` says when it is ready, with where to fetch it. One at a time, and one a week unless the installation says otherwise (`EXPORT_EVERY_DAYS`).").
		Viewer(apidoc.SignedIn).
		StepUp().
		Sends(http.StatusCreated, handlers.ExportItem{}).
		Sends(http.StatusForbidden, libs_responses.Refusal{}, "`step_up_required`: the viewer must prove who they are again first (`/reauth`)").
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`active`: one is being made").
		Sends(http.StatusTooManyRequests, handlers.TooSoon{}, "`too_soon`: it is not yet time to ask again, and it says when")
	exports.GET("/:id", h.Read).
		Summary("How an export is getting on").
		Describe("Once it is ready, with an address to fetch it from, good for a few minutes and made afresh each time this is read.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ExportRequest{}).
		Unbound().
		Sends(http.StatusOK, handlers.ExportItem{}).
		Refuses(http.StatusNotFound, "the viewer has no such export")
	exports.DELETE("/:id", h.Delete).
		Summary("Give an export up").
		Describe("Stops it being made, or deletes its archive once it is.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ExportRequest{}).
		Unbound().
		SendsNothing(http.StatusNoContent, "given up, or it was over already").
		Refuses(http.StatusNotFound, "the viewer has no such export")

	// Bringing a person's lists in from a file (roadmap row 124).
	im := r.imports
	imports := api.Group("imports", r.authorize)
	imports.GET("", im.Browse).
		Summary("The viewer's imports").
		Describe("The latest five, newest first, each with how far it has got.").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.ImportList{})
	imports.POST("", im.Add, middleware.BodyLimit(UploadLimit)).
		Summary("Read a file to import").
		Describe("Another Anahita's download, as a zip, or a single CSV, Mastodon's or Misskey's: who the viewer follows, blocks and mutes, and what they saved. Read at once and kept, `ready`, with how many of each kind; nothing is done until it is started. One at a time; at most 20,000 lines and 20 MB.").
		Viewer(apidoc.SignedIn).
		Scope(constants.OAuthScopeSocialgraphWrite).
		Takes(&handlers.ImportUpload{}).
		Sends(http.StatusCreated, handlers.ImportItem{}).
		Sends(http.StatusBadRequest, libs_responses.Refusal{}, "the file could not be read: `empty`, `unreadable`, `too_large`, `too_many_lines`, `not_supported` (lists, blocked servers), or `kind_unknown`: a CSV with no header, whose kind the viewer is to say").
		Sends(http.StatusForbidden, libs_responses.Refusal{}, "`insufficient_scope`").
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`active`: one is waiting or running")
	imports.GET("/:id", im.Read).
		Summary("How an import is getting on").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ImportRequest{}).
		Unbound().
		Sends(http.StatusOK, handlers.ImportItem{}).
		Refuses(http.StatusNotFound, "the viewer has no such import")
	imports.POST("/:id/start", im.Start).
		Summary("Start an import").
		Describe("With the kinds the viewer chose; the lines of the others are skipped. Each line is then done as the viewer would by hand, a line a second, in the background.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ImportStart{}).
		Sends(http.StatusOK, handlers.ImportItem{}).
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`not_ready`: started or given up already").
		Refuses(http.StatusNotFound, "the viewer has no such import")
	imports.GET("/:id/failed", im.Failed).
		Summary("What an import could not do").
		Describe("A CSV: the kind, who or what, the name, and why in a word.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ImportRequest{}).
		Unbound().
		Media("text/csv").
		Sends(http.StatusOK, apidoc.File{}, "the file").
		Refuses(http.StatusNotFound, "the viewer has no such import")
	imports.DELETE("/:id", im.Delete).
		Summary("Give an import up").
		Describe("What it has not done is not done; what it did stays.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ImportRequest{}).
		Unbound().
		SendsNothing(http.StatusNoContent, "given up, or it was over already").
		Refuses(http.StatusNotFound, "the viewer has no such import")

	return doc
}
