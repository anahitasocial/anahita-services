package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"actor/data/services"
	libconstants "anahita-libs/constants"
	"anahita-libs/data/entities"
	libservices "anahita-libs/data/services"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// GET /events/ — what is on. The graph does the choosing; what is pinned
// here is what the handler hands it: the window, read from the address,
// online and following, and nothing asked of it for a visitor who wants
// what the people they follow are going to, since they follow nobody.

type fakeDiscoverEvents struct {
	libservices.EventsGRPCInterface

	calls  int
	filter entities.EventDiscoverFilter
}

func (f *fakeDiscoverEvents) BrowseOpen(viewer *entities.Viewer, filter entities.EventDiscoverFilter, limit uint64, offset uint64) ([]int64, uint64, error) {
	f.calls++
	f.filter = filter
	return []int64{}, 0, nil
}

func discover(t *testing.T, events *fakeDiscoverEvents, viewerID int64, query string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	e.Validator = validators.NewRequestValidator(validator.New())
	req := httptest.NewRequest(http.MethodGet, "/events/?"+query, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	viewer := entities.NewViewer()
	viewer.ID = viewerID
	c.Set(libconstants.ViewerContextKey, viewer)

	h := NewEventHandler(&Env{Srv: &services.Services{EventsGRPC: events}})
	if err := h.Discover(c); err != nil {
		t.Fatalf("discover: %v", err)
	}
	return rec
}

func TestDiscoverHandsTheGraphItsChoices(t *testing.T) {
	events := &fakeDiscoverEvents{}
	rec := discover(t, events, 42, "from=2026-11-07T08:00:00Z&to=2026-11-09T08:00:00Z&online=true&following=true")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if events.calls != 1 {
		t.Fatalf("the graph was asked %d times, want once", events.calls)
	}
	got := events.filter
	from := time.Date(2026, 11, 7, 8, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)
	if got.From == nil || !got.From.Equal(from) || got.To == nil || !got.To.Equal(to) {
		t.Errorf("window = %v to %v, want %v to %v", got.From, got.To, from, to)
	}
	if !got.Online || !got.Following {
		t.Errorf("online %v, following %v; want both", got.Online, got.Following)
	}
}

func TestDiscoverWithNoChoicesIsEverything(t *testing.T) {
	events := &fakeDiscoverEvents{}
	discover(t, events, 0, "")

	got := events.filter
	if got.From != nil || got.To != nil || got.Online || got.Following {
		t.Errorf("filter = %+v, want none", got)
	}
}

func TestDiscoverRefusesATimeItCannotRead(t *testing.T) {
	events := &fakeDiscoverEvents{}
	rec := discover(t, events, 42, "from=this-weekend")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if events.calls != 0 {
		t.Error("the graph was asked anyway")
	}
}

func TestDiscoverFollowingForAVisitorIsEmpty(t *testing.T) {
	events := &fakeDiscoverEvents{}
	rec := discover(t, events, 0, "following=true")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if events.calls != 0 {
		t.Error("the graph was asked for a visitor's following")
	}
	var body struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Data) != 0 {
		t.Errorf("body %s (err %v), want an empty list", rec.Body.String(), err)
	}
}
