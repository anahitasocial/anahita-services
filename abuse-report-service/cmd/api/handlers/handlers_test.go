package handlers

import (
	"anahita-libs/apidoc"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"abusereport/cmd/api/permissions"
	"abusereport/config"
	"abusereport/constants"
	"abusereport/data/entities"
	"abusereport/data/reasons"
	"abusereport/data/repositories"
	"abusereport/data/services"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"
	libs_services "anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// ---------------------------------------------------------------------
// Doubles
// ---------------------------------------------------------------------

// fakeCases keeps cases in memory, with the rules the real tables enforce:
// one open case per target, one report per person per case.
type fakeCases struct {
	mu      sync.Mutex
	nextID  int64
	cases   map[int64]*entities.Case
	reports map[int64][]*entities.Report

	// removedReporters are the people whose reports were asked to be removed.
	removedReporters []int64
}

func newFakeCases() *fakeCases {
	return &fakeCases{cases: map[int64]*entities.Case{}, reports: map[int64][]*entities.Report{}}
}

func (f *fakeCases) AddReport(target entities.Case, report entities.Report) (int64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var open *entities.Case
	for _, c := range f.cases {
		if c.TargetID == target.TargetID && c.Status == constants.CaseStatusOpen {
			open = c
		}
	}
	if open == nil {
		f.nextID++
		target.ID = f.nextID
		target.Status = constants.CaseStatusOpen
		target.FirstReportedAt = time.Now()
		open = &target
		f.cases[open.ID] = open
	}

	for _, existing := range f.reports[open.ID] {
		if existing.ReporterID == report.ReporterID {
			existing.ReasonKey, existing.Details = report.ReasonKey, report.Details
			return open.ID, false, nil
		}
	}

	report.CaseID = open.ID
	report.ID = int64(len(f.reports[open.ID]) + 1)
	report.CreatedAt = time.Now()
	f.reports[open.ID] = append(f.reports[open.ID], &report)
	open.ReportCount++
	open.LastReportedAt = time.Now()

	return open.ID, true, nil
}

func (f *fakeCases) Browse(status, reasonKey string, limit, offset uint64) ([]*entities.Case, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var list []*entities.Case
	for id := int64(1); id <= f.nextID; id++ {
		c := f.cases[id]
		if c == nil || c.Status != status {
			continue
		}
		if reasonKey != "" && !f.gave(c.ID, reasonKey) {
			continue
		}
		list = append(list, c)
	}
	return list, nil
}

func (f *fakeCases) gave(caseID int64, reasonKey string) bool {
	for _, report := range f.reports[caseID] {
		if report.ReasonKey == reasonKey {
			return true
		}
	}
	return false
}

func (f *fakeCases) Count(status, reasonKey string) (uint64, error) {
	list, _ := f.Browse(status, reasonKey, 0, 0)
	return uint64(len(list)), nil
}

func (f *fakeCases) Read(id int64) (*entities.Case, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if c, ok := f.cases[id]; ok {
		return c, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeCases) BrowseReports(caseID int64) ([]*entities.Report, error) {
	return f.reports[caseID], nil
}

func (f *fakeCases) BrowseForTarget(targetID, except int64) ([]*entities.Case, error) {
	var list []*entities.Case
	for _, c := range f.cases {
		if c.TargetID == targetID && c.ID != except {
			list = append(list, c)
		}
	}
	return list, nil
}

func (f *fakeCases) ReasonCounts(caseIDs []int64) ([]entities.ReasonCount, error) {
	var counts []entities.ReasonCount
	for _, id := range caseIDs {
		tally := map[string]int{}
		for _, report := range f.reports[id] {
			tally[report.ReasonKey]++
		}
		for key, n := range tally {
			counts = append(counts, entities.ReasonCount{CaseID: id, ReasonKey: key, Count: n})
		}
	}
	return counts, nil
}

func (f *fakeCases) Resolve(id int64, status, action, note string, by int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	c := f.cases[id]
	if c == nil || c.Status != constants.CaseStatusOpen {
		return false, nil
	}
	now := time.Now()
	c.Status, c.DecisionAction, c.DecisionNote, c.DecidedBy, c.DecidedAt = status, action, note, by, &now
	return true, nil
}

func (f *fakeCases) Reopen(id int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	c := f.cases[id]
	if c == nil || c.Status == constants.CaseStatusOpen {
		return false, nil
	}
	for _, other := range f.cases {
		if other.TargetID == c.TargetID && other.Status == constants.CaseStatusOpen {
			return false, repositories.ErrOpenCaseExists
		}
	}
	c.Status, c.DecisionAction, c.DecisionNote, c.DecidedBy, c.DecidedAt = constants.CaseStatusOpen, "", "", 0, nil
	return true, nil
}

func (f *fakeCases) RemoveReportsBy(reporterID int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.removedReporters = append(f.removedReporters, reporterID)
	return 0, nil
}

func (f *fakeCases) ForgetAddresses(before time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeCases) MarkTargetGone(targetID int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var closed int64
	now := time.Now()
	for _, c := range f.cases {
		if c.TargetID != targetID {
			continue
		}
		c.TargetGoneAt = &now
		if c.Status == constants.CaseStatusOpen {
			c.Status, c.DecisionAction, c.DecidedAt = constants.CaseStatusActioned, constants.ActionDeleted, &now
			closed++
		}
	}
	return closed, nil
}

// fakeGraph holds the nodes that exist, and which of them a viewer who is
// not an administrator may see.
type fakeGraph struct {
	libs_services.GraphNodeGRPCInterface

	nodes  map[int64]*libs_entities.AnahitaNode
	hidden map[int64]bool
}

func (f *fakeGraph) FilterVisible(ids []int64, viewer *libs_entities.Viewer) ([]int64, error) {
	var visible []int64
	for _, id := range ids {
		if _, exists := f.nodes[id]; exists && !f.hidden[id] {
			visible = append(visible, id)
		}
	}
	return visible, nil
}

func (f *fakeGraph) Read(id int64) (*libs_entities.AnahitaNode, error) {
	return f.nodes[id], nil
}

func (f *fakeGraph) BatchRead(req *libs_services.GraphNodeBatchReadRequest) ([]*libs_entities.AnahitaNode, error) {
	var list []*libs_entities.AnahitaNode
	for _, id := range req.IDs {
		if node, ok := f.nodes[id]; ok {
			list = append(list, node)
		}
	}
	return list, nil
}

type memoryCounter struct {
	mu     sync.Mutex
	counts map[string]int64
}

func (m *memoryCounter) Incr(key string, _ time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[key]++
	return m.counts[key], nil
}

func (m *memoryCounter) Del(keys ...string) error { return nil }

// ---------------------------------------------------------------------
// The cast
// ---------------------------------------------------------------------

const (
	adaID       = int64(1) // a member, and the author of the note
	graceID     = int64(2) // a member
	moderatorID = int64(9) // an administrator
	noteID      = int64(100)
	hiddenID    = int64(101) // a note Grace cannot see
	hashtagID   = int64(30)
)

func node(id int64, nodeType, name string, createdBy int64) *libs_entities.AnahitaNode {
	n := libs_entities.NewAnahitaNode()
	n.ID, n.Type, n.Name, n.CreatedBy = id, nodeType, name, createdBy
	return n
}

type fixture struct {
	t     *testing.T
	e     *echo.Echo
	h     *Handlers
	cases *fakeCases
	graph *fakeGraph
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	const person = "node.actor.person-service.person.v1"
	const note = "node.medium.text-service.note.v1"

	graph := &fakeGraph{
		nodes: map[int64]*libs_entities.AnahitaNode{
			adaID:       node(adaID, person, "Ada", 0),
			graceID:     node(graceID, person, "Grace", 0),
			moderatorID: node(moderatorID, person, "Moderator", 0),
			noteID:      node(noteID, note, "A note", adaID),
			hiddenID:    node(hiddenID, note, "A private note", adaID),
			hashtagID:   node(hashtagID, "node.tag.hashtag-service.hashtag.v1", "gardens", 0),
		},
		hidden: map[int64]bool{hiddenID: true},
	}

	cases := newFakeCases()
	e := echo.New()
	e.Validator = validators.NewRequestValidator(validator.New())

	env := &Env{
		Config:  &config.Config{},
		Perms:   permissions.NewPermissions(),
		Repos:   &repositories.Repositories{Cases: cases},
		Srv:     &services.Services{GraphNodeGRPC: graph},
		Reasons: reasons.NewSource(""),
		Limiter: &memoryCounter{counts: map[string]int64{}},
	}

	return &fixture{t: t, e: e, h: NewHandlers(env), cases: cases, graph: graph}
}

func member(id int64) *libs_entities.Viewer {
	v := libs_entities.NewViewer()
	v.ID = id
	v.PersonType = libs_constants.PersonTypeRegistered
	v.Scopes = []string{libs_constants.OAuthScopeAccountWrite, libs_constants.OAuthScopeActorsRead, libs_constants.OAuthScopeActorsWrite}
	return v
}

func admin() *libs_entities.Viewer {
	v := member(moderatorID)
	v.PersonType = libs_constants.PersonTypeAdmin
	return v
}

func (f *fixture) call(method, path, body string, viewer *libs_entities.Viewer, params map[string]string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	f.t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := f.e.NewContext(req, rec)
	if viewer != nil {
		c.Set(libs_constants.ViewerContextKey, viewer)
	}
	for name, value := range params {
		c.SetParamNames(name)
		c.SetParamValues(value)
	}

	if err := handler(c); err != nil {
		f.t.Fatalf("%s %s: %v", method, path, err)
	}
	apidoc.Conforms(f.t, "../../../../docs/api/abuse-report-service.json", method+" "+path, rec)
	return rec
}

func (f *fixture) report(viewer *libs_entities.Viewer, target int64, reason, details string) *httptest.ResponseRecorder {
	f.t.Helper()
	body, _ := json.Marshal(map[string]any{"target_id": target, "reason_key": reason, "details": details})
	return f.call(http.MethodPost, "/abuse-reports", string(body), viewer, nil, f.h.Reports.Add)
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, want, rec.Body.String())
	}
}

// ---------------------------------------------------------------------
// Reporting
// ---------------------------------------------------------------------

func TestTwoPeopleReportingOneNodeMakeOneCase(t *testing.T) {
	f := newFixture(t)
	third := member(3)

	wantStatus(t, f.report(member(graceID), noteID, "hate_racism", ""), http.StatusCreated)
	wantStatus(t, f.report(third, noteID, "violence", "It names somebody."), http.StatusCreated)

	open, _ := f.cases.Browse(constants.CaseStatusOpen, "", 10, 0)
	if len(open) != 1 || open[0].ReportCount != 2 {
		t.Fatalf("%d open cases; the first has %d reports", len(open), open[0].ReportCount)
	}
	if open[0].TargetOwnerID != adaID || open[0].TargetType != "node.medium.text-service.note.v1" {
		t.Errorf("case target: owner %d type %q", open[0].TargetOwnerID, open[0].TargetType)
	}
	if got := f.cases.reports[open[0].ID][0].CreatedIP; got == "" {
		t.Error("the report did not record where it came from")
	}
}

func TestReportingAgainUpdatesAndAnswers200(t *testing.T) {
	f := newFixture(t)

	wantStatus(t, f.report(member(graceID), noteID, "spam", ""), http.StatusCreated)
	wantStatus(t, f.report(member(graceID), noteID, "harassment", "Changed my mind."), http.StatusOK)

	reports := f.cases.reports[1]
	if len(reports) != 1 || reports[0].ReasonKey != "harassment" {
		t.Errorf("reports: %+v", reports)
	}
}

// A node the reporter cannot see, and one that does not exist, get the same
// answer. Otherwise reporting would reveal what is hidden.
func TestReportingWhatYouCannotSeeIsNotFound(t *testing.T) {
	f := newFixture(t)

	hidden := f.report(member(graceID), hiddenID, "spam", "")
	missing := f.report(member(graceID), 424242, "spam", "")

	wantStatus(t, hidden, http.StatusNotFound)
	wantStatus(t, missing, http.StatusNotFound)
	if hidden.Body.String() != missing.Body.String() {
		t.Error("a hidden node and a missing one are answered differently")
	}
	if len(f.cases.cases) != 0 {
		t.Error("a case was opened")
	}
}

func TestYouCannotReportYourselfOrYourOwnPost(t *testing.T) {
	f := newFixture(t)

	wantStatus(t, f.report(member(adaID), adaID, "spam", ""), http.StatusBadRequest)
	wantStatus(t, f.report(member(adaID), noteID, "spam", ""), http.StatusBadRequest)
	if len(f.cases.cases) != 0 {
		t.Error("a case was opened")
	}
}

func TestTheReasonMustExistAndApplyToTheKindOfNode(t *testing.T) {
	f := newFixture(t)
	grace := member(graceID)

	// Impersonation is for people and groups.
	wantStatus(t, f.report(grace, noteID, "impersonation", ""), http.StatusBadRequest)
	wantStatus(t, f.report(grace, adaID, "impersonation", ""), http.StatusCreated)

	wantStatus(t, f.report(grace, noteID, "no_such_reason", ""), http.StatusBadRequest)

	// "Something else" has to be explained.
	wantStatus(t, f.report(grace, noteID, "other", "   "), http.StatusBadRequest)
	wantStatus(t, f.report(grace, noteID, "other", "It is a scam."), http.StatusCreated)

	// A hashtag has nobody who answers for it.
	wantStatus(t, f.report(grace, hashtagID, "spam", ""), http.StatusCreated)
	if hashtagCase, _ := f.cases.Read(3); hashtagCase.TargetOwnerID != 0 {
		t.Errorf("a hashtag's case names an owner: %d", hashtagCase.TargetOwnerID)
	}
}

func TestReportsAreLimitedPerPerson(t *testing.T) {
	f := newFixture(t)
	grace := member(graceID)

	for n := 1; n <= constants.ReportsPerHour; n++ {
		if rec := f.report(grace, noteID, "spam", ""); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("report %d was limited", n)
		}
	}
	wantStatus(t, f.report(grace, noteID, "spam", ""), http.StatusTooManyRequests)

	// Somebody else is not affected.
	wantStatus(t, f.report(member(3), noteID, "spam", ""), http.StatusCreated)
}

func TestOnlySignedInPeopleWithTheScopeMayReport(t *testing.T) {
	f := newFixture(t)

	guest := libs_entities.NewViewer()
	wantStatus(t, f.report(guest, noteID, "spam", ""), http.StatusForbidden)

	noScope := member(graceID)
	noScope.Scopes = nil
	wantStatus(t, f.report(noScope, noteID, "spam", ""), http.StatusForbidden)

	wantStatus(t, f.call(http.MethodGet, "/abuse-reports/reasons", "", guest, nil, f.h.Reasons.Browse), http.StatusForbidden)
}

func TestReasonsAreFilteredByTypeAndTranslated(t *testing.T) {
	f := newFixture(t)

	decode := func(rec *httptest.ResponseRecorder) map[string]map[string]any {
		var body struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		byKey := map[string]map[string]any{}
		for _, reason := range body.Data {
			byKey[reason["key"].(string)] = reason
		}
		return byKey
	}

	forNote := decode(f.call(http.MethodGet, "/abuse-reports/reasons?type=node.medium.text-service.note.v1&lang=fr-FR",
		"", member(graceID), nil, f.h.Reasons.Browse))

	if _, offered := forNote["impersonation"]; offered {
		t.Error("impersonation is offered for a note")
	}
	if forNote["hate_racism"]["label"] != "Racisme" {
		t.Errorf("label = %v, want the French one", forNote["hate_racism"]["label"])
	}
	if forNote["other"]["requires_details"] != true {
		t.Error(`"other" does not say it needs an explanation`)
	}

	forPerson := decode(f.call(http.MethodGet, "/abuse-reports/reasons?type=node.actor.person-service.person.v1",
		"", member(graceID), nil, f.h.Reasons.Browse))
	if _, offered := forPerson["impersonation"]; !offered {
		t.Error("impersonation is not offered for a person")
	}
}

// ---------------------------------------------------------------------
// Administrators
// ---------------------------------------------------------------------

func TestMembersCannotSeeOrDecideCases(t *testing.T) {
	f := newFixture(t)
	f.report(member(graceID), noteID, "spam", "")
	grace := member(graceID)
	id := map[string]string{"id": "1"}

	wantStatus(t, f.call(http.MethodGet, "/abuse-reports/cases", "", grace, nil, f.h.Cases.Browse), http.StatusForbidden)
	wantStatus(t, f.call(http.MethodGet, "/abuse-reports/cases/summary", "", grace, nil, f.h.Cases.Summary), http.StatusForbidden)
	wantStatus(t, f.call(http.MethodGet, "/abuse-reports/cases/1", "", grace, id, f.h.Cases.Read), http.StatusForbidden)
	wantStatus(t, f.call(http.MethodPost, "/abuse-reports/cases/1/resolve", `{"status":"dismissed"}`, grace, id, f.h.Cases.Resolve), http.StatusForbidden)
	wantStatus(t, f.call(http.MethodPost, "/abuse-reports/cases/1/reopen", "", grace, id, f.h.Cases.Reopen), http.StatusForbidden)

	if c, _ := f.cases.Read(1); c.Status != constants.CaseStatusOpen {
		t.Error("a member changed a case")
	}
}

func TestAnAdministratorSeesTheCaseWithItsReportsAndReporters(t *testing.T) {
	f := newFixture(t)
	f.report(member(graceID), noteID, "hate_racism", "Second paragraph.")
	f.report(member(3), noteID, "hate_racism", "")
	id := map[string]string{"id": "1"}

	summary := f.call(http.MethodGet, "/abuse-reports/cases/summary", "", admin(), nil, f.h.Cases.Summary)
	if !strings.Contains(summary.Body.String(), `"open":1`) {
		t.Errorf("summary: %s", summary.Body.String())
	}

	var list struct {
		Data []struct {
			ID          int64 `json:"id"`
			ReportCount int   `json:"report_count"`
			Target      struct {
				Name string `json:"name"`
			} `json:"target"`
			TargetOwner struct {
				Name string `json:"name"`
			} `json:"target_owner"`
			Reasons []struct {
				Key   string `json:"key"`
				Label string `json:"label"`
				Count int    `json:"count"`
			} `json:"reasons"`
		} `json:"data"`
		Pagination struct {
			Total uint64 `json:"total"`
		} `json:"pagination"`
	}
	rec := f.call(http.MethodGet, "/abuse-reports/cases?status=open", "", admin(), nil, f.h.Cases.Browse)
	wantStatus(t, rec, http.StatusOK)
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Pagination.Total != 1 || len(list.Data) != 1 {
		t.Fatalf("list: %s", rec.Body.String())
	}
	row := list.Data[0]
	if row.ReportCount != 2 || row.Target.Name != "A note" || row.TargetOwner.Name != "Ada" {
		t.Errorf("row: %+v", row)
	}
	if len(row.Reasons) != 1 || row.Reasons[0].Label != "Racism" || row.Reasons[0].Count != 2 {
		t.Errorf("reasons: %+v", row.Reasons)
	}

	var detail struct {
		Reports []struct {
			Reporter struct {
				Name string `json:"name"`
			} `json:"reporter"`
			ReasonLabel string `json:"reason_label"`
			Details     string `json:"details"`
		} `json:"reports"`
	}
	rec = f.call(http.MethodGet, "/abuse-reports/cases/1", "", admin(), id, f.h.Cases.Read)
	wantStatus(t, rec, http.StatusOK)
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Reports) != 2 || detail.Reports[0].Reporter.Name != "Grace" ||
		detail.Reports[0].Details != "Second paragraph." || detail.Reports[0].ReasonLabel != "Racism" {
		t.Errorf("detail: %s", rec.Body.String())
	}
	// Where a report came from is kept, and not handed out.
	if strings.Contains(rec.Body.String(), "created_ip") {
		t.Error("the case page exposes reporters' network addresses")
	}
}

func TestDecidingACaseAndReportingAgainOpensANewOne(t *testing.T) {
	f := newFixture(t)
	f.report(member(graceID), noteID, "spam", "")
	id := map[string]string{"id": "1"}

	wantStatus(t, f.call(http.MethodPost, "/abuse-reports/cases/1/resolve",
		`{"status":"sideways"}`, admin(), id, f.h.Cases.Resolve), http.StatusBadRequest)

	wantStatus(t, f.call(http.MethodPost, "/abuse-reports/cases/1/resolve",
		`{"status":"dismissed","action":"deleted","note":"Not spam."}`, admin(), id, f.h.Cases.Resolve), http.StatusNoContent)

	decided, _ := f.cases.Read(1)
	if decided.Status != constants.CaseStatusDismissed || decided.DecidedBy != moderatorID || decided.DecisionNote != "Not spam." {
		t.Errorf("decision: %+v", decided)
	}
	// A dismissed case had nothing done about it, whatever the client sent.
	if decided.DecisionAction != constants.ActionNone {
		t.Errorf("action = %q, want none", decided.DecisionAction)
	}

	// Somebody else deciding the same case is told it is already decided.
	wantStatus(t, f.call(http.MethodPost, "/abuse-reports/cases/1/resolve",
		`{"status":"actioned","action":"deleted"}`, admin(), id, f.h.Cases.Resolve), http.StatusConflict)

	wantStatus(t, f.report(member(3), noteID, "spam", ""), http.StatusCreated)
	if len(f.cases.cases) != 2 {
		t.Fatalf("%d cases, want a second one", len(f.cases.cases))
	}

	// The first cannot be reopened while the second is open.
	wantStatus(t, f.call(http.MethodPost, "/abuse-reports/cases/1/reopen", "", admin(), id, f.h.Cases.Reopen), http.StatusConflict)

	// And the second's page shows the first as history.
	rec := f.call(http.MethodGet, "/abuse-reports/cases/2", "", admin(), map[string]string{"id": "2"}, f.h.Cases.Read)
	var detail struct {
		Previous []struct {
			ID int64 `json:"id"`
		} `json:"previous"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &detail)
	if len(detail.Previous) != 1 || detail.Previous[0].ID != 1 {
		t.Errorf("history: %s", rec.Body.String())
	}
}

func TestAnActionedCaseWithNoActionNamedIsRecordedAsOther(t *testing.T) {
	f := newFixture(t)
	f.report(member(graceID), noteID, "spam", "")

	wantStatus(t, f.call(http.MethodPost, "/abuse-reports/cases/1/resolve",
		`{"status":"actioned"}`, admin(), map[string]string{"id": "1"}, f.h.Cases.Resolve), http.StatusNoContent)

	if c, _ := f.cases.Read(1); c.DecisionAction != constants.ActionOther {
		t.Errorf("action = %q", c.DecisionAction)
	}
}

// ---------------------------------------------------------------------
// The reported thing going away
// ---------------------------------------------------------------------

func TestADeletedPostClosesItsCaseAndADeletedPersonDoesNot(t *testing.T) {
	f := newFixture(t)
	f.report(member(graceID), noteID, "spam", "")
	f.report(member(graceID), adaID, "impersonation", "")

	// A person who deletes their account can still restore it.
	_ = f.h.Events.Handle(&event.EventPayload{Name: "node.actor.person-service.person.deleted.v1", ObjectID: adaID})
	if c, _ := f.cases.Read(2); c.Status != constants.CaseStatusOpen {
		t.Error("a soft-deleted person's case was closed")
	}
	if len(f.cases.removedReporters) != 0 {
		t.Error("a soft-deleted person's own reports were removed; they can still come back")
	}

	_ = f.h.Events.Handle(&event.EventPayload{Name: "node.medium.text-service.note.deleted.v1", ObjectID: noteID})
	if c, _ := f.cases.Read(1); c.Status != constants.CaseStatusActioned || c.DecisionAction != constants.ActionDeleted || c.TargetGoneAt == nil {
		t.Errorf("the post's case: %+v", c)
	}

	_ = f.h.Events.Handle(&event.EventPayload{Name: "node.actor.person-service.person.purged.v1", ObjectID: adaID})
	if c, _ := f.cases.Read(2); c.Status != constants.CaseStatusActioned {
		t.Error("a purged person's case stayed open")
	}

	// Only the purge removes what the person reported, and a deleted post is
	// not somebody who could have filed a report.
	if got := f.cases.removedReporters; len(got) != 1 || got[0] != adaID {
		t.Errorf("reports were removed for %v, want only the purged person", got)
	}

	// Anything else is ignored.
	if err := f.h.Events.Handle(&event.EventPayload{Name: "node.medium.text-service.note.added.v1", ObjectID: noteID}); err != nil {
		t.Error(err)
	}
}

// The event may never arrive. Opening the case is the check that cannot be
// lost.
func TestOpeningACaseNoticesThatTheTargetIsGone(t *testing.T) {
	f := newFixture(t)
	f.report(member(graceID), noteID, "spam", "")

	delete(f.graph.nodes, noteID)

	rec := f.call(http.MethodGet, "/abuse-reports/cases/1", "", admin(), map[string]string{"id": "1"}, f.h.Cases.Read)
	wantStatus(t, rec, http.StatusOK)

	if !strings.Contains(rec.Body.String(), `"target_gone":true`) || !strings.Contains(rec.Body.String(), `"status":"actioned"`) {
		t.Errorf("the page does not say the target is gone: %s", rec.Body.String())
	}
	if c, _ := f.cases.Read(1); c.TargetGoneAt == nil {
		t.Error("it was not recorded")
	}
}

func TestCasesCanBeFilteredByReason(t *testing.T) {
	f := newFixture(t)
	f.report(member(graceID), noteID, "spam", "")
	f.report(member(graceID), adaID, "impersonation", "")

	count := func(query string) int {
		rec := f.call(http.MethodGet, "/abuse-reports/cases"+query, "", admin(), nil, f.h.Cases.Browse)
		wantStatus(t, rec, http.StatusOK)
		var body struct {
			Data       []map[string]any `json:"data"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Pagination.Total != len(body.Data) {
			t.Fatalf("%s: total %d, rows %d", query, body.Pagination.Total, len(body.Data))
		}
		return len(body.Data)
	}

	if got := count("?status=open"); got != 2 {
		t.Errorf("no reason: %d cases", got)
	}
	if got := count("?status=open&reason=spam"); got != 1 {
		t.Errorf("spam: %d cases", got)
	}
	// A key that is on nobody's list is not an error.
	if got := count("?status=open&reason=retired_reason"); got != 0 {
		t.Errorf("an unknown reason: %d cases", got)
	}

	// The number on the menu counts every open case, whatever the filter.
	summary := f.call(http.MethodGet, "/abuse-reports/cases/summary", "", admin(), nil, f.h.Cases.Summary)
	if !strings.Contains(summary.Body.String(), `"open":2`) {
		t.Errorf("summary: %s", summary.Body.String())
	}
}
