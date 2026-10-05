package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"

	"github.com/labstack/echo/v4"
)

// The other half of both restrictions: somebody who IS signed in is never
// refused and never sent a preview. A members-only site that turned its own
// members away would be the worse failure of the two.

// asSignedIn makes SetViewer see a signed-in person for the length of a test.
func asSignedIn(t *testing.T, id int64) {
	t.Helper()

	previous := authenticate
	authenticate = func(echo.Context, ConfigInterface, *JWKSClient) (*entities.Viewer, error) {
		viewer := entities.NewViewer()
		viewer.ID = id
		viewer.Authenticated = true
		viewer.PersonType = constants.PersonTypeRegistered
		return viewer, nil
	}
	t.Cleanup(func() { authenticate = previous })
}

// request runs one GET through SetViewer and whatever follows it.
func request(t *testing.T, target string, handler echo.HandlerFunc, more ...echo.MiddlewareFunc) (*httptest.ResponseRecorder, *entities.Viewer) {
	t.Helper()

	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, target, nil), rec)

	var seen *entities.Viewer
	chain := echo.HandlerFunc(func(c echo.Context) error {
		seen, _ = c.Get(constants.ViewerContextKey).(*entities.Viewer)
		return handler(c)
	})
	for i := len(more) - 1; i >= 0; i-- {
		chain = more[i](chain)
	}

	if err := SetViewer(nil, nil)(chain)(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	return rec, seen
}

func longPost(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"data": []map[string]any{{
			"id":            1,
			"body":          strings.Repeat("word ", 100),
			"portrait_urls": map[string]any{"small": "s", "large": "l", "original": "o"},
		}},
		"pagination": map[string]any{"offset": 20, "limit": 50, "total": 400},
	})
}

func TestASignedInPersonReadsAMembersOnlySite(t *testing.T) {
	restore := policy.SetReadAccessForTest(policy.ReadRegistered)
	defer restore()
	asSignedIn(t, 7)

	rec, viewer := request(t, "/notes/5/?start=20&limit=50", longPost)

	if rec.Code != http.StatusOK {
		t.Fatalf("a member was answered %d on their own site: %s", rec.Code, rec.Body.String())
	}
	if viewer == nil || viewer.ID != 7 {
		t.Errorf("the handler was given %+v, not the person who is signed in", viewer)
	}
}

func TestASignedInPersonIsNotSentAPreview(t *testing.T) {
	restore := policy.SetReadAccessForTest(policy.ReadPreview)
	defer restore()
	asSignedIn(t, 7)

	// A later page and a large one, both of which a visitor is refused.
	rec, _ := request(t, "/notes/5/?start=20&limit=50", longPost)

	if rec.Code != http.StatusOK {
		t.Fatalf("answered %d", rec.Code)
	}
	body := rec.Body.String()
	for _, cut := range []string{`"truncated"`, `"more"`, "…"} {
		if strings.Contains(body, cut) {
			t.Errorf("a signed-in person was sent a preview (%s): %s", cut, body[:200])
		}
	}
	for _, whole := range []string{`"total":400`, `"original":"o"`, `"large":"l"`} {
		if !strings.Contains(body, whole) {
			t.Errorf("the answer is missing %s", whole)
		}
	}
}

// Comments and follower lists are left out of a preview for visitors only.
func TestASignedInPersonReadsCommentsOnAPreviewSite(t *testing.T) {
	restore := policy.SetReadAccessForTest(policy.ReadPreview)
	defer restore()
	asSignedIn(t, 7)

	reached := false
	rec, _ := request(t, "/comments/100/", func(c echo.Context) error {
		reached = true
		return c.NoContent(http.StatusOK)
	}, MembersInPreview())

	if !reached || rec.Code != http.StatusOK {
		t.Errorf("reached=%v, answered %d", reached, rec.Code)
	}
}

// And on a members-only site those routes are refused by SetViewer itself,
// before MembersInPreview is ever asked.
func TestAVisitorCannotReadCommentsOnAMembersOnlySite(t *testing.T) {
	restore := policy.SetReadAccessForTest(policy.ReadRegistered)
	defer restore()

	reached := false
	rec, _ := request(t, "/comments/100/", func(c echo.Context) error {
		reached = true
		return c.NoContent(http.StatusOK)
	}, MembersInPreview())

	if reached || rec.Code != http.StatusUnauthorized {
		t.Errorf("reached=%v, answered %d", reached, rec.Code)
	}
}

// The routes a preview leaves out have to say so themselves, on the route:
// SetViewer cannot know that one list is of comments and another of posts.
// These are the three, and a fourth such route has to be added here.
func TestTheRoutesAPreviewLeavesOutSaySo(t *testing.T) {
	for file, routes := range map[string][]string{
		"comment-service":     {`notes.GET("/"`},
		"socialgraph-service": {`"/:actor_id/followers/"`, `"/:actor_id/leaders/"`},
	} {
		source, err := os.ReadFile(filepath.Join("..", "..", file, "cmd", "api", "routes.go"))
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}

		for _, route := range routes {
			found := false
			for _, line := range strings.Split(string(source), "\n") {
				if strings.Contains(line, route) && !strings.HasPrefix(strings.TrimSpace(line), "//") {
					found = true
					if !strings.Contains(line, "MembersInPreview()") {
						t.Errorf("%s serves %s to visitors on a preview site: %s",
							file, route, strings.TrimSpace(line))
					}
				}
			}
			if !found {
				t.Errorf("%s: could not find the route %s; has it moved?", file, route)
			}
		}
	}
}
