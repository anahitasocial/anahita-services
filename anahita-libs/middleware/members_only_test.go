package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"

	"github.com/labstack/echo/v4"
)

// A members-only site is enforced in one place, SetViewer, and these two
// tests are what make that enough: the first that it does refuse, the second
// that nothing goes around it.

func anonymousRequest(t *testing.T) (*httptest.ResponseRecorder, bool, *entities.Viewer) {
	t.Helper()

	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/notes/5/", nil), httptest.NewRecorder())
	rec := c.Response().Writer.(*httptest.ResponseRecorder)

	reached := false
	var seen *entities.Viewer
	handler := SetViewer(nil, nil)(func(c echo.Context) error {
		reached = true
		seen, _ = c.Get(constants.ViewerContextKey).(*entities.Viewer)
		return c.NoContent(http.StatusOK)
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	return rec, reached, seen
}

func TestSetViewerAdmitsAnonymousReadersOnAnOpenSite(t *testing.T) {
	rec, reached, viewer := anonymousRequest(t)

	if !reached || rec.Code != http.StatusOK {
		t.Fatalf("an anonymous request was stopped on an open site: %d", rec.Code)
	}
	if viewer == nil || viewer.ID != 0 {
		t.Errorf("the handler was not given an anonymous viewer: %+v", viewer)
	}
}

func TestSetViewerRefusesAnonymousReadersOnAMembersOnlySite(t *testing.T) {
	restore := policy.SetReadAccessForTest(policy.ReadRegistered)
	defer restore()

	rec, reached, _ := anonymousRequest(t)

	if reached {
		t.Fatal("the handler ran for somebody who is not signed in")
	}
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), ErrMembersOnly) {
		t.Errorf("answered %d %s, want 401 %s", rec.Code, rec.Body.String(), ErrMembersOnly)
	}
}

// Every route, in every service, either requires signing in (Authorize) or
// goes through SetViewer. One with neither would serve people who are not
// signed in without the members-only setting ever being asked.
//
// Read out of the routes files rather than by starting the services. What is
// checked is how each route is REGISTERED: with a middleware on the line, on
// its group, or on the group it was made from.
//
// auth-service is the exception, and is held to a list: its open routes are
// how anybody gets in, so they must stay open on a members-only site, and a
// new one has to be added here on purpose.
// betweenServices is the routes one service asks of another. They have no
// viewer because nobody is behind them: they are not under an address the
// gateway sends anybody to, they refuse a request that came through it,
// and they give out nothing a member wrote. Each is held here by name, so
// that a new one is added on purpose and not by leaving a middleware off.
var betweenServices = map[string][]string{
	// The card of a link in a post from another server, asked for by
	// activitypub-service. A preview of a public page, never of anything
	// on this installation.
	"link-preview-service": {"GET /internal/link-previews"},
}

func TestEveryRouteNamesItsViewer(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "*", "cmd", "api", "routes.go"))
	if err != nil || len(files) < 10 {
		t.Fatalf("found %d routes files (%v); has the layout changed?", len(files), err)
	}

	open := map[string][]string{}
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		service := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(file))))
		if routes := openRoutes(string(source)); len(routes) > 0 {
			open[service] = routes
		}
	}

	for service, routes := range open {
		if service == "auth-service" {
			continue
		}
		// What one service asks of another, and nobody outside is sent to.
		routes = slices.DeleteFunc(routes, func(route string) bool {
			return slices.Contains(betweenServices[service], route)
		})
		if len(routes) == 0 {
			continue
		}
		t.Errorf("%s has routes with neither Authorize nor SetViewer, which a "+
			"members-only site would still serve to anybody:\n  %s",
			service, strings.Join(routes, "\n  "))
	}

	// The ways in. Nothing here returns anything a member wrote.
	allowed := []string{
		"/auth/is/", "/password/", "/invites/accept", "/signup",
		"/.well-known/", "/nodeinfo/", "/oauth/",
	}
	for _, route := range open["auth-service"] {
		path := route[strings.Index(route, " ")+1:]
		known := false
		for _, prefix := range allowed {
			known = known || strings.HasPrefix(path, prefix)
		}
		if !known {
			t.Errorf("auth-service serves %s with no viewer middleware, and it is "+
				"not one of the ways in. If it is meant to be open, add it to the "+
				"list in this test.", route)
		}
	}
	if len(open["auth-service"]) == 0 {
		t.Error("found no open routes in auth-service; the pattern has stopped matching")
	}
}

var (
	groupDecl = regexp.MustCompile(`(\w+)\s*:?=\s*(\w+)\.Group\("([^"]*)"(?:,\s*([^)]*))?\)`)
	groupUse  = regexp.MustCompile(`(\w+)\.Use\(([^)]*)\)`)
	routeDecl = regexp.MustCompile(`(?m)^\s*(\w+)\.(GET|POST|PATCH|DELETE|PUT|Any)\(\s*"([^"]*)"\s*,([^\n]*)`)
)

func guards(middleware string) bool {
	lower := strings.ToLower(middleware)
	return strings.Contains(lower, "authorize") || strings.Contains(lower, "setviewer")
}

// openRoutes lists the routes in one routes file that have no viewer
// middleware on the line, on their group, or on a group above it.
func openRoutes(source string) []string {
	type group struct {
		prefix  string
		guarded bool
	}
	groups := map[string]group{}

	for _, m := range groupDecl.FindAllStringSubmatch(source, -1) {
		parent := groups[m[2]]
		groups[m[1]] = group{
			prefix:  parent.prefix + "/" + strings.Trim(m[3], "/"),
			guarded: parent.guarded || guards(m[4]),
		}
	}
	for _, m := range groupUse.FindAllStringSubmatch(source, -1) {
		if g, ok := groups[m[1]]; ok && guards(m[2]) {
			g.guarded = true
			groups[m[1]] = g
		}
	}

	var open []string
	for _, m := range routeDecl.FindAllStringSubmatch(source, -1) {
		g := groups[m[1]]
		if g.guarded || guards(m[4]) {
			continue
		}
		path := strings.ReplaceAll(g.prefix+"/"+strings.TrimLeft(m[3], "/"), "//", "/")
		open = append(open, m[2]+" "+path)
	}

	sort.Strings(open)
	return open
}

// The reader of routes files, on a file that has one of each case.
func TestOpenRoutesFindsOnlyTheUnguarded(t *testing.T) {
	source := `
	notes := e.Group("notes")
	notes.GET("/:id", r.handler.Read, r.setViewer)
	notes.POST("/", r.handler.Add, r.authorize)
	notes.GET("/open", r.handler.Open)

	likes := e.Group("likes")
	likes.Use(r.authorize)
	likes.GET("/", r.handler.Browse)

	reports := e.Group("reports", r.authorize)
	cases := reports.Group("/cases")
	cases.GET("/:id", r.handler.Case)
	// notes.GET("/old", r.handler.Old)
`
	got := strings.Join(openRoutes(source), ", ")
	if got != "GET /notes/open" {
		t.Errorf("open routes: %q, want only GET /notes/open", got)
	}
}
