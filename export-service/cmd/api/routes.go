package main

import (
	"net/http"

	"export/cmd/api/handlers"

	"anahita-libs/apidoc"
	libs_responses "anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.ExportsHandler
	authorize echo.MiddlewareFunc
}

func NewRoutes(handler *handlers.ExportsHandler, authorize echo.MiddlewareFunc) *Routes {
	return &Routes{handler: handler, authorize: authorize}
}

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/export-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	h := r.handler
	doc := apidoc.NewService("export-service", "Data exports",
		"A person's copy of their own data: asking for one, seeing how it is getting on, fetching it and giving it up (data-export.md). Optional: an installation may not run it.")
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

	return doc
}
