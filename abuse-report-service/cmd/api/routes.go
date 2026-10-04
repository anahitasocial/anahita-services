package main

import (
	"abusereport/cmd/api/handlers"

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
func (r *Routes) Init(e *echo.Echo) {
	reports := e.Group("abuse-reports", r.authorize)

	reports.GET("/reasons", r.handler.Reasons.Browse)
	reports.POST("", r.handler.Reports.Add)
	reports.POST("/", r.handler.Reports.Add)

	// "summary" before ":id", so it is not read as a case's id.
	reports.GET("/cases", r.handler.Cases.Browse)
	reports.GET("/cases/summary", r.handler.Cases.Summary)
	reports.GET("/cases/:id", r.handler.Cases.Read)
	reports.POST("/cases/:id/resolve", r.handler.Cases.Resolve)
	reports.POST("/cases/:id/reopen", r.handler.Cases.Reopen)
}
