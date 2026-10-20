package main

import (
	"net/http"

	"abusereport/cmd/api/handlers"
	"abusereport/requests"
	"abusereport/responses"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	libs_responses "anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
}

func NewRoutes(handler *handlers.Handlers, authorize echo.MiddlewareFunc) *Routes {
	return &Routes{handler: handler, authorize: authorize}
}

// Init registers the routes. Every one needs a signed-in viewer: only
// members report, and only administrators read what was reported. Which of
// the two a route is for is checked in its handler.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("abuse-report-service", "Abuse reports",
		"Reporting a post, a reply or a profile, and the administrators' queue of what was reported. Optional: an installation may not run it (abuse-reports.md).")
	api := doc.On(e)
	reports := api.Group("abuse-reports", r.authorize)
	report := libs_constants.OAuthScopeAccountWrite
	readCases := libs_constants.OAuthScopeActorsRead
	decide := libs_constants.OAuthScopeActorsWrite

	reports.GET("/reasons", r.handler.Reasons.Browse).
		Summary("What a thing may be reported for").
		Describe("The installation's list, worded in the language asked for.").
		Viewer(apidoc.SignedIn).
		Scope(report).
		Takes(&requests.ReasonsBrowse{}).
		Sends(http.StatusOK, libs_responses.Unpaged[responses.Reason]{})
	add := func(route *apidoc.Route) {
		route.
			Summary("Report something").
			Describe("Nobody is told: the person reported never learns of the report or who made it, and administrators find it in their queue. A thing the viewer cannot see answers exactly as one that does not exist, so that reporting is no way to find out what is hidden. Twenty reports an hour at most. With `reason_key` `rule`, it breaks one of the rules everybody ticked, named by its title in `rule`: the site's (`rule_profile_id` 0, from `GET /site`) or those of the group or event it is in, or is (its id).").
			Viewer(apidoc.SignedIn).
			Scope(report).
			Takes(&requests.ReportAdd{}).
			SendsNothing(http.StatusCreated, "reported").
			SendsNothing(http.StatusOK, "the viewer had reported it already; their report was updated").
			Sends(http.StatusBadRequest, libs_responses.Refusal{}, "`own_content`, it is the viewer's own; `reason_not_applicable`, the reason is not for this kind of thing; `details_required`, the reason needs the reporter's own words; `rule_required`, `rule` is empty; `rule_not_applicable`, it is not one of that profile's rules, or the thing is not in that profile; or not well formed, without a body").
			Refuses(http.StatusNotFound, "there is no such thing that the viewer can see").
			Refuses(http.StatusTooManyRequests, "too many reports in the last hour")
	}
	reports.POST("", r.handler.Reports.Add).With(add)
	reports.POST("/", r.handler.Reports.Add).With(add)

	// "summary" before ":id", so it is not read as a case's id.
	reports.GET("/cases", r.handler.Cases.Browse).
		Summary("Cases").
		Describe("What was reported, one case for each thing, however many reported it.").
		Viewer(apidoc.Admin).
		Scope(readCases).
		Takes(&requests.CasesBrowse{}).
		Sends(http.StatusOK, libs_responses.List[responses.Case]{})
	reports.GET("/cases/summary", r.handler.Cases.Summary).
		Summary("How many cases are open").
		Viewer(apidoc.Admin).
		Scope(readCases).
		Sends(http.StatusOK, responses.CaseSummary{})
	reports.GET("/cases/:id", r.handler.Cases.Read).
		Summary("One case").
		Describe("With each report, and the earlier cases about the same thing.").
		Viewer(apidoc.Admin).
		Scope(readCases).
		Takes(&requests.CaseRead{}).
		Sends(http.StatusOK, responses.CaseDetail{}).
		Refuses(http.StatusNotFound, "there is no such case")
	reports.POST("/cases/:id/resolve", r.handler.Cases.Resolve).
		Summary("Decide a case").
		Viewer(apidoc.Admin).
		Scope(decide).
		Takes(&requests.CaseResolve{}).
		SendsNothing(http.StatusNoContent, "decided").
		Refuses(http.StatusNotFound, "there is no such case").
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`already_decided`")
	reports.POST("/cases/:id/reopen", r.handler.Cases.Reopen).
		Summary("Reopen a case").
		Viewer(apidoc.Admin).
		Scope(decide).
		Takes(&requests.CaseReopen{}).
		SendsNothing(http.StatusNoContent, "open again").
		Refuses(http.StatusNotFound, "there is no such case").
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`already_open`; or `newer_case_open`, a later case about the same thing is open")

	return doc
}
