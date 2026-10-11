package main

import (
	"recommendation/cmd/api/handlers"

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
func (r *Routes) Init(e *echo.Echo) {
	recommendations := e.Group("recommendations", r.authorize)

	recommendations.GET("/actors", r.handler.Actors.Browse)

	recommendations.GET("/preferences", r.handler.Preferences.Read)
	recommendations.PUT("/preferences", r.handler.Preferences.Edit)

	recommendations.GET("/dismissals", r.handler.Dismissals.Browse)
	recommendations.POST("/dismissals", r.handler.Dismissals.Add)
	recommendations.DELETE("/dismissals/:actor_id", r.handler.Dismissals.Delete)
}
