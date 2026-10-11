package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"recommendation/constants"
	"recommendation/data/repositories"
	"recommendation/data/services"
	"recommendation/scoring"

	an_const "anahita-libs/constants"
	"anahita-libs/data/entities"
	an_services "anahita-libs/data/services"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// The endpoints, over signals and a graph held in memory. What the signals
// find and whom they leave out is tested against a real database beside
// their queries. Here: who is answered, what the viewer's choices do, and
// that nothing is sent of a profile the graph says they may not see.

type fakeSignals struct {
	found map[string][]scoring.Hit
	asked int
	// What it was asked with, last.
	useActivity   bool
	onlyColdStart bool
	kind          string
}

func (s *fakeSignals) Find(viewerID int64, kind string, useActivity, onlyColdStart bool, now time.Time) (map[string][]scoring.Hit, error) {
	s.asked++
	s.kind, s.useActivity, s.onlyColdStart = kind, useActivity, onlyColdStart
	return s.found, nil
}

func (s *fakeSignals) LastActive(ids []int64) (map[int64]time.Time, error) {
	active := map[int64]time.Time{}
	for _, id := range ids {
		active[id] = time.Now()
	}
	return active, nil
}

type fakePreferences struct {
	rows map[int64]*repositories.Preferences
}

func (p *fakePreferences) Read(personID int64) (*repositories.Preferences, error) {
	if row, ok := p.rows[personID]; ok {
		copied := *row
		return &copied, nil
	}
	return repositories.DefaultPreferences(), nil
}

func (p *fakePreferences) Write(personID int64, prefs *repositories.Preferences) error {
	copied := *prefs
	p.rows[personID] = &copied
	return nil
}

func (p *fakePreferences) Forget(personID int64) error {
	delete(p.rows, personID)
	return nil
}

type fakeDismissals struct {
	rows map[int64][]int64
}

func (d *fakeDismissals) Add(personID, actorID int64) error {
	d.rows[personID] = append(d.rows[personID], actorID)
	return nil
}

func (d *fakeDismissals) Remove(personID, actorID int64) error {
	kept := []int64{}
	for _, id := range d.rows[personID] {
		if id != actorID {
			kept = append(kept, id)
		}
	}
	d.rows[personID] = kept
	return nil
}

func (d *fakeDismissals) List(personID int64) ([]int64, error) {
	return d.rows[personID], nil
}

func (d *fakeDismissals) Forget(actorID int64) error {
	delete(d.rows, actorID)
	return nil
}

// fakeGraph says which nodes may be seen, and reads them.
type fakeGraph struct {
	an_services.GraphNodeGRPCInterface
	hidden map[int64]bool
}

func (g *fakeGraph) FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error) {
	visible := []int64{}
	// Answered backwards: the graph promises no order.
	for i := len(ids) - 1; i >= 0; i-- {
		if !g.hidden[ids[i]] {
			visible = append(visible, ids[i])
		}
	}
	return visible, nil
}

func node(id int64) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = id
	n.Type = constants.PersonType
	n.Name = fmt.Sprintf("P%d", id)
	return n
}

func (g *fakeGraph) BatchRead(req *an_services.GraphNodeBatchReadRequest) ([]*entities.AnahitaNode, error) {
	list := []*entities.AnahitaNode{}
	for _, id := range req.IDs {
		list = append(list, node(id))
	}
	return list, nil
}

type fakeActors struct {
	an_services.ActorsGRPCInterface
	read []int64
}

func (a *fakeActors) BatchRead(req *an_services.ActorGRPCBatchReadRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	a.read = append(a.read, req.IDs...)
	list := []*entities.AnahitaNode{}
	for _, id := range req.IDs {
		list = append(list, node(id))
	}
	return list, nil
}

type fixture struct {
	t        *testing.T
	echo     *echo.Echo
	handlers *Handlers
	signals  *fakeSignals
	prefs    *fakePreferences
	graph    *fakeGraph
	actors   *fakeActors
	dismiss  *fakeDismissals
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	f := &fixture{
		t:       t,
		echo:    echo.New(),
		signals: &fakeSignals{found: map[string][]scoring.Hit{}},
		prefs:   &fakePreferences{rows: map[int64]*repositories.Preferences{}},
		graph:   &fakeGraph{hidden: map[int64]bool{}},
		actors:  &fakeActors{},
		dismiss: &fakeDismissals{rows: map[int64][]int64{}},
	}
	f.echo.Validator = validators.NewRequestValidator(validator.New())
	f.handlers = NewHandlers(&Env{
		Repos: &repositories.Repositories{Signals: f.signals, Preferences: f.prefs, Dismissals: f.dismiss},
		Srv:   &services.Services{GraphNodeGRPC: f.graph, ActorsGRPC: f.actors},
	})

	return f
}

func person(id int64) *entities.Viewer {
	viewer := entities.NewViewer()
	if id > 0 {
		viewer.ID = id
		viewer.Authenticated = true
		viewer.PersonType = "registered"
	}
	return viewer
}

func (f *fixture) call(handler echo.HandlerFunc, method, target string, viewer *entities.Viewer, body string, params ...string) (int, map[string]any) {
	f.t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	c := f.echo.NewContext(req, rec)
	c.Set(an_const.ViewerContextKey, viewer)
	if len(params) == 2 {
		c.SetParamNames(params[0])
		c.SetParamValues(params[1])
	}

	if err := handler(c); err != nil {
		f.t.Fatalf("handler: %v", err)
	}

	answer := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
			f.t.Fatalf("answer is not JSON: %s", rec.Body.String())
		}
	}

	return rec.Code, answer
}

func suggested(answer map[string]any) string {
	ids := []int64{}
	for _, item := range answer["items"].([]any) {
		ids = append(ids, int64(item.(map[string]any)["actor_id"].(float64)))
	}
	return fmt.Sprint(ids)
}

func TestNobodyIsSuggestedToAVisitor(t *testing.T) {
	f := newFixture(t)

	for name, handler := range map[string]echo.HandlerFunc{
		"suggestions": f.handlers.Actors.Browse,
		"preferences": f.handlers.Preferences.Read,
		"dismissals":  f.handlers.Dismissals.Browse,
	} {
		if status, _ := f.call(handler, http.MethodGet, "/", person(0), ""); status != http.StatusForbidden {
			t.Errorf("%s answered a visitor with %d", name, status)
		}
	}
	if f.signals.asked != 0 {
		t.Errorf("signals were run for a visitor")
	}
}

func TestSuggestionsAreRankedGatedAndCut(t *testing.T) {
	f := newFixture(t)
	f.signals.found = map[string][]scoring.Hit{
		constants.ReasonFollowedByLeaders: {{ID: 4, Count: 3, IDs: []int64{2, 3, 9}}, {ID: 5, Count: 2, IDs: []int64{2, 3}}, {ID: 6, Count: 1, IDs: []int64{2}}},
		constants.ReasonFollowsYou:        {{ID: 7, Count: 1}},
	}
	// The graph says the viewer may not see 5, nor 9 who follows 4.
	f.graph.hidden = map[int64]bool{5: true, 9: true}

	status, answer := f.call(f.handlers.Actors.Browse, http.MethodGet, "/?limit=2", person(1), "")
	if status != http.StatusOK {
		t.Fatalf("answered %d", status)
	}

	// 4 (9 points), then 5 (6) which is hidden, then 7 (4), then 6 (3):
	// the two best that may be seen.
	if got := suggested(answer); got != "[4 7]" {
		t.Errorf("suggested %s", got)
	}
	if got := fmt.Sprint(f.actors.read); got != "[4 7]" {
		t.Errorf("profiles read: %s", got)
	}

	first := answer["items"].([]any)[0].(map[string]any)
	if first["actor"].(map[string]any)["name"] != "P4" {
		t.Errorf("sent without the profile: %v", first["actor"])
	}
	reason := first["reasons"].([]any)[0].(map[string]any)
	if reason["type"] != constants.ReasonFollowedByLeaders || reason["count"] != float64(3) {
		t.Errorf("the reason for 4: %v", reason)
	}

	// What the reasons name is sent to say them with, less what may not
	// be seen.
	related := []int64{}
	for _, each := range answer["related"].([]any) {
		related = append(related, int64(each.(map[string]any)["id"].(float64)))
	}
	if len(related) != 2 || related[0]+related[1] != 5 {
		t.Errorf("related: %v", related)
	}
}

func TestChoicesDecideWhatIsLookedFor(t *testing.T) {
	f := newFixture(t)
	f.signals.found = map[string][]scoring.Hit{constants.ReasonFeatured: {{ID: 4, Count: 1}}}

	// Asked not to be suggested groups: none, and none looked for.
	f.call(f.handlers.Preferences.Edit, http.MethodPut, "/", person(1), `{"suggest_groups":false,"use_activity":false}`)
	status, answer := f.call(f.handlers.Actors.Browse, http.MethodGet, "/?kind=group", person(1), "")
	if status != http.StatusOK || suggested(answer) != "[]" || f.signals.asked != 0 {
		t.Errorf("groups answered %d with %s after %d searches", status, suggested(answer), f.signals.asked)
	}

	// People still, without what the viewer does, and as somebody new.
	_, answer = f.call(f.handlers.Actors.Browse, http.MethodGet, "/?context=onboarding", person(1), "")
	if suggested(answer) != "[4]" || f.signals.kind != constants.KindPerson || f.signals.useActivity || !f.signals.onlyColdStart {
		t.Errorf("suggested %s, asked with kind %q, activity %v, cold start %v",
			suggested(answer), f.signals.kind, f.signals.useActivity, f.signals.onlyColdStart)
	}

	// A choice not sent is left as it was.
	_, prefs := f.call(f.handlers.Preferences.Edit, http.MethodPut, "/", person(1), `{"discoverable":false}`)
	if prefs["suggest_groups"] != false || prefs["use_activity"] != false || prefs["discoverable"] != false || prefs["suggest_people"] != true {
		t.Errorf("choices after a second change: %v", prefs)
	}
	// Somebody else's are their own.
	_, others := f.call(f.handlers.Preferences.Read, http.MethodGet, "/", person(2), "")
	if others["discoverable"] != true || others["suggest_groups"] != true {
		t.Errorf("somebody else's choices: %v", others)
	}

	// A kind there is not is refused.
	if status, _ := f.call(f.handlers.Actors.Browse, http.MethodGet, "/?kind=event", person(1), ""); status != http.StatusBadRequest {
		t.Errorf("events answered %d", status)
	}
}

func TestTurningASuggestionDown(t *testing.T) {
	f := newFixture(t)

	if status, _ := f.call(f.handlers.Dismissals.Add, http.MethodPost, "/", person(1), `{"actor_id":4}`); status != http.StatusNoContent {
		t.Fatalf("answered %d", status)
	}
	f.call(f.handlers.Dismissals.Add, http.MethodPost, "/", person(1), `{"actor_id":5}`)
	f.graph.hidden = map[int64]bool{5: true}

	// Listed back, less one the viewer can no longer see.
	_, listed := f.call(f.handlers.Dismissals.Browse, http.MethodGet, "/", person(1), "")
	if list := listed["data"].([]any); len(list) != 1 || list[0].(map[string]any)["id"] != float64(4) {
		t.Errorf("listed %v", listed["data"])
	}
	// Somebody else's list is their own.
	if _, theirs := f.call(f.handlers.Dismissals.Browse, http.MethodGet, "/", person(2), ""); len(theirs["data"].([]any)) != 0 {
		t.Errorf("somebody else was shown %v", theirs["data"])
	}

	// Nobody turns themselves down, or nothing.
	for _, body := range []string{`{"actor_id":1}`, `{"actor_id":0}`, `{}`} {
		if status, _ := f.call(f.handlers.Dismissals.Add, http.MethodPost, "/", person(1), body); status != http.StatusBadRequest {
			t.Errorf("%s answered %d", body, status)
		}
	}

	if status, _ := f.call(f.handlers.Dismissals.Delete, http.MethodDelete, "/", person(1), "", "actor_id", "4"); status != http.StatusNoContent {
		t.Errorf("taking it back answered %d", status)
	}
	if got := fmt.Sprint(f.dismiss.rows[1]); got != "[5]" {
		t.Errorf("still turned down: %s", got)
	}
}
