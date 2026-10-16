package main

import (
	"net/http"

	"recommendation/cmd/api/handlers"
	"recommendation/data/repositories"

	"anahita-libs/apidoc"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
}

func NewRoutes(handler *handlers.Handlers, authorize echo.MiddlewareFunc) *Routes {
	return &Routes{handler: handler, authorize: authorize}
}

// Init registers the routes. Every one needs somebody signed in: a
// suggestion is made to a person, about that person.
//
// These paths are the contract. An installation that wants its suggestions
// chosen another way runs another service that answers them. See
// docs/recommendations.md.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("recommendation-service", "Recommendations",
		"Who the viewer might follow, with the reasons, worked out from the graph by rules and no model; and the viewer's own choices about it. Optional: an installation may not run it, or may run another service that answers these routes (recommendations.md).")
	api := doc.On(e)
	recommendations := api.Group("recommendations", r.authorize)

	recommendations.GET("/actors", r.handler.Actors.Browse).
		Summary("Who the viewer might follow").
		Describe("People, groups and events, best first, each with why. Nobody the viewer follows already, nobody on either side of a block with them, nobody they dismissed, and nobody who asked not to be suggested: the whole list is under Who is never suggested in recommendations.md.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.ActorsRequest{}).
		Sends(http.StatusOK, handlers.ActorsResponse{})
	// The hashtags offered in the Interests step of signing up.
	recommendations.GET("/interests", r.handler.Interests.Browse).
		Summary("Hashtags to pick from when signing up").
		Describe("The ones on the most public posts lately, the most used first, looking further back until enough are found: the last three months, then the year, then all of it.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.InterestsRequest{}).
		Sends(http.StatusOK, responses.Unpaged[repositories.Interest]{})

	recommendations.GET("/preferences", r.handler.Preferences.Read).
		Summary("The viewer's choices").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.PreferencesResponse{})
	recommendations.PUT("/preferences", r.handler.Preferences.Edit).
		Summary("Change the viewer's choices").
		Describe("What is sent is set, and what is left out is left as it is. Answers with them all.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.PreferencesRequest{}).
		Sends(http.StatusOK, handlers.PreferencesResponse{}).
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, "`invalid_weight`: a reason there is not, or a weight out of range; nothing is changed")

	recommendations.GET("/dismissals", r.handler.Dismissals.Browse).
		Summary("Whom the viewer is not interested in").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, responses.Unpaged[*responses.ActorBrief]{})
	recommendations.POST("/dismissals", r.handler.Dismissals.Add).
		Summary("Not interested in this one").
		Describe("They are not suggested to the viewer again.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.DismissalAddRequest{}).
		SendsNothing(http.StatusNoContent, "done")
	recommendations.DELETE("/dismissals/:actor_id", r.handler.Dismissals.Delete).
		Summary("Take that back").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.DismissalRemoveRequest{}).
		SendsNothing(http.StatusNoContent, "done")

	return doc
}
