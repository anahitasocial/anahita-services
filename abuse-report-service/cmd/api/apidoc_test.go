package main

import (
	"testing"

	"abusereport/cmd/api/handlers"

	"anahita-libs/apidoc"

	"github.com/labstack/echo/v4"
)

// THE RULE THIS GUARDS: every route is described, and
// docs/api/abuse-report-service.json is what the routes describe now. Run
// `make api_docs` to write it again after a change.
func TestTheAPIReference(t *testing.T) {
	e := echo.New()
	pass := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	doc := NewRoutes(&handlers.Handlers{}, pass).Init(e)
	apidoc.Check(t, e, doc, "../../../docs/api/abuse-report-service.json")
}
