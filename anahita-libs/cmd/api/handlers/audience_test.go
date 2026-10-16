package handlers

import (
	"anahita-libs/apidoc"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"anahita-libs/cmd/api/permissions"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"

	"github.com/labstack/echo/v4"
)

// What a client is told when an audience is refused: 422, the error code, and
// the audiences that are allowed, so it can offer the right ones.

func refusal(t *testing.T, answer func(c echo.Context) error) (int, map[string]any) {
	t.Helper()

	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodPost, "/notes/8/", nil), rec)

	if err := answer(c); err != nil {
		t.Fatalf("answering: %v", err)
	}
	// As posting a note answers it.
	apidoc.Conforms(t, "../../../../docs/api/text-service.json", "POST /notes/8/", rec)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("the answer is not JSON: %s", rec.Body.String())
	}
	return rec.Code, body
}

func TestARefusedAudienceSaysWhichAreAllowed(t *testing.T) {
	group := entities.NewAnahitaNode()
	group.ID, group.Type = 8, "node.actor.group-service.group.v1"

	post := entities.NewAnahitaNode()
	post.Owner, post.OwnerID, post.CreatedBy = group, 8, 2

	viewer := entities.NewViewer()
	viewer.ID, viewer.PersonType = 2, constants.PersonTypeRegistered

	ctx := &permissions.MediumAccessContext{Viewer: viewer, Medium: post}

	status, body := refusal(t, func(c echo.Context) error {
		return RefuseAudience(c, ctx)
	})

	if status != http.StatusUnprocessableEntity || body["error"] != "access_not_allowed" {
		t.Errorf("answered %d %v", status, body)
	}
	allowed, _ := body["allowed"].([]any)
	if len(allowed) != 3 || allowed[0] != "public" || allowed[2] != "followers" {
		t.Errorf("allowed: %v, want public, registered, followers", allowed)
	}
}

func TestARefusedProfileAccessSaysWhichAreAllowed(t *testing.T) {
	restore := policy.SetForTest("registered")
	defer restore()

	status, body := refusal(t, func(c echo.Context) error {
		return RefuseAccess(c, policy.Allowed([]string{"public", "registered", "followers", "admins"}))
	})

	if status != http.StatusUnprocessableEntity || body["error"] != "access_not_allowed" {
		t.Errorf("answered %d %v", status, body)
	}
	allowed, _ := body["allowed"].([]any)
	if len(allowed) != 3 || allowed[0] != "registered" {
		t.Errorf("allowed: %v, want everything but public", allowed)
	}
}
