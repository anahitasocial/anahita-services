package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"actor/data/services"
	libconstants "anahita-libs/constants"
	"anahita-libs/data/entities"
	libservices "anahita-libs/data/services"

	"github.com/labstack/echo/v4"
)

// /people/:id/content-preferences (roadmap row 122, phase 2): a person's
// choices are theirs alone, a choice that is not one is refused, and the
// kinds not named stay as they were.

type fakeContentPreferences struct {
	libservices.ActorsGRPCInterface

	choices map[string]string
	saves   int
}

func (f *fakeContentPreferences) ReadContentPreferences(personID int64) (map[string]string, error) {
	return entities.WarningChoices(f.choices), nil
}

func (f *fakeContentPreferences) SaveContentPreferences(personID int64, choices map[string]string) (map[string]string, error) {
	f.saves++
	f.choices = entities.WarningChoicesKept(choices)
	return entities.WarningChoices(choices), nil
}

func contentPreferences(t *testing.T, actors *fakeContentPreferences, method, body string, viewerID int64) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(method, "/people/7/content-preferences", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("7")

	viewer := entities.NewViewer()
	viewer.ID = viewerID
	viewer.PersonType = libconstants.PersonTypeRegistered
	c.Set(libconstants.ViewerContextKey, viewer)

	h := NewPersonHandler(&Env{Srv: &services.Services{ActorsGRPC: actors}})
	handle := h.ReadContentPreferences
	if method == http.MethodPatch {
		handle = h.EditContentPreferences
	}
	if err := handle(c); err != nil {
		t.Fatalf("content preferences: %v", err)
	}
	answers.Handler(handle, rec)
	return rec
}

func TestContentPreferencesAreTheirOwn(t *testing.T) {
	actors := &fakeContentPreferences{}
	if rec := contentPreferences(t, actors, http.MethodGet, "", 8); rec.Code != http.StatusForbidden {
		t.Errorf("read by another: status = %d, want 403", rec.Code)
	}
	if rec := contentPreferences(t, actors, http.MethodPatch, `{"choices":{"graphic":"show"}}`, 8); rec.Code != http.StatusForbidden {
		t.Errorf("changed by another: status = %d, want 403", rec.Code)
	}
	if actors.saves != 0 {
		t.Error("somebody else's choices were saved")
	}
}

func TestContentPreferencesRefuseWhatIsNotAChoice(t *testing.T) {
	for _, body := range []string{
		`{"choices":{"graphic":"blur"}}`,
		`{"choices":{"gore":"hide"}}`,
	} {
		actors := &fakeContentPreferences{}
		rec := contentPreferences(t, actors, http.MethodPatch, body, 7)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "content_preference_invalid") {
			t.Errorf("%s: status = %d, body %s", body, rec.Code, rec.Body.String())
		}
		if actors.saves != 0 {
			t.Errorf("%s: saved anyway", body)
		}
	}
}

func TestContentPreferencesKeepWhatIsNotNamed(t *testing.T) {
	actors := &fakeContentPreferences{choices: map[string]string{"adult": "hide"}}
	rec := contentPreferences(t, actors, http.MethodPatch, `{"choices":{"graphic":"show"}}`, 7)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		Choices map[string]string `json:"choices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"adult": "hide", "suggestive": "warn", "graphic": "show", "nudity": "warn", "other": "warn"}
	for kind, choice := range want {
		if got.Choices[kind] != choice {
			t.Errorf("%s: got %q, want %q", kind, got.Choices[kind], choice)
		}
	}
}
