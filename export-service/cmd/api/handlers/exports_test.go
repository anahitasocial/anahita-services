package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/sessions"

	"export/config"
	"export/data/repositories"

	"github.com/labstack/echo/v4"
)

// The endpoints, over uploads and a session kept in memory.

type fakeExports struct {
	repositories.ExportsInterface
	rows     map[string]*repositories.Export
	lastMade time.Time
	active   bool
}

func (f *fakeExports) Add(export *repositories.Export) error {
	if f.active {
		return repositories.ErrActive
	}
	export.Status = repositories.StatusPending
	export.CreatedAt = time.Now().UTC()
	f.rows[export.ID] = export
	return nil
}

func (f *fakeExports) Read(id string) (*repositories.Export, error) { return f.rows[id], nil }

func (f *fakeExports) OfPerson(personID int64) ([]*repositories.Export, error) {
	var mine []*repositories.Export
	for _, export := range f.rows {
		if export.PersonID == personID {
			mine = append(mine, export)
		}
	}
	return mine, nil
}

func (f *fakeExports) LastMade(personID int64) (time.Time, error) { return f.lastMade, nil }

func (f *fakeExports) ToCancelled(id string) error {
	f.rows[id].Status = repositories.StatusCancelled
	return nil
}

type fakeTransfers struct {
	services.FileTransfersInterface
}

func (fakeTransfers) PresignDownload(path, name string) (string, error) {
	return "https://storage.example/" + path + "?as=" + name, nil
}

type fakeFiles struct {
	services.FilesGRPCInterface
	deleted []string
}

func (f *fakeFiles) Delete(req *services.FileDeleteRequest) error {
	f.deleted = append(f.deleted, req.Path)
	return nil
}

// proofs is the session store: who has just proved who they are.
type proofs struct {
	kept map[string]bool
}

func (p *proofs) Exists(key string) (bool, error) { return p.kept[key], nil }

func (p *proofs) Del(keys ...string) error {
	for _, key := range keys {
		delete(p.kept, key)
	}
	return nil
}

type fixture struct {
	t       *testing.T
	echo    *echo.Echo
	handler *exportsHandler
	exports *fakeExports
	files   *fakeFiles
	proofs  *proofs
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{
		t:       t,
		echo:    echo.New(),
		exports: &fakeExports{rows: map[string]*repositories.Export{}},
		files:   &fakeFiles{},
		proofs:  &proofs{kept: map[string]bool{}},
	}
	f.handler = NewExportsHandler(&Env{
		Config:    &config.Config{EveryDays: 7, KeepDays: 7},
		Exports:   f.exports,
		Transfers: fakeTransfers{},
		Files:     f.files,
		Session:   f.proofs,
	})
	return f
}

// prove records that this person has just proved who they are, as
// auth-service does.
func (f *fixture) prove(personID int64) {
	f.proofs.kept[sessions.StepUpKey(personID)] = true
}

func (f *fixture) call(handler echo.HandlerFunc, method string, personID int64, id string) (int, map[string]any) {
	f.t.Helper()

	rec := httptest.NewRecorder()
	c := f.echo.NewContext(httptest.NewRequest(method, "/", nil), rec)
	viewer := entities.NewViewer()
	viewer.ID = personID
	viewer.Authenticated = true
	c.Set(constants.ViewerContextKey, viewer)
	if id != "" {
		c.SetParamNames("id")
		c.SetParamValues(id)
	}
	if err := handler(c); err != nil {
		f.t.Fatalf("handler: %v", err)
	}

	answer := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &answer)
	}
	return rec.Code, answer
}

func TestAnExportIsOnlyForSomebodyWhoJustProvedWhoTheyAre(t *testing.T) {
	f := newFixture(t)

	status, answer := f.call(f.handler.Add, http.MethodPost, 5, "")
	if status != http.StatusForbidden || answer["error"] != "step_up_required" || len(f.exports.rows) != 0 {
		t.Fatalf("without proof: %d %v, %d made", status, answer, len(f.exports.rows))
	}

	// Somebody else's proof is not theirs.
	f.prove(6)
	if status, _ := f.call(f.handler.Add, http.MethodPost, 5, ""); status != http.StatusForbidden {
		t.Fatalf("with another's proof: %d", status)
	}

	f.prove(5)
	status, answer = f.call(f.handler.Add, http.MethodPost, 5, "")
	if status != http.StatusCreated || answer["status"] != repositories.StatusPending {
		t.Fatalf("with proof: %d %v", status, answer)
	}

	// One proof, one export.
	f.exports.rows = map[string]*repositories.Export{}
	if status, _ := f.call(f.handler.Add, http.MethodPost, 5, ""); status != http.StatusForbidden {
		t.Errorf("the same proof again: %d", status)
	}

	// With nothing to check against, nobody is let through.
	f.handler.Session = nil
	if status, _ := f.call(f.handler.Add, http.MethodPost, 5, ""); status != http.StatusInternalServerError {
		t.Errorf("with no session store: %d", status)
	}
}

func TestOneAWeekAndOneAtATime(t *testing.T) {
	f := newFixture(t)

	f.prove(5)
	f.exports.lastMade = time.Now().UTC().Add(-3 * 24 * time.Hour)
	status, answer := f.call(f.handler.Add, http.MethodPost, 5, "")
	if status != http.StatusTooManyRequests || answer["error"] != "too_soon" || answer["next_at"] == nil {
		t.Fatalf("three days on: %d %v", status, answer)
	}
	// Being told to wait does not use the proof up.
	f.exports.lastMade = time.Now().UTC().Add(-8 * 24 * time.Hour)
	if status, _ := f.call(f.handler.Add, http.MethodPost, 5, ""); status != http.StatusCreated {
		t.Fatalf("eight days on, with the same proof: %d", status)
	}

	f.prove(5)
	f.exports.lastMade = time.Time{}
	f.exports.active = true
	status, answer = f.call(f.handler.Add, http.MethodPost, 5, "")
	if status != http.StatusConflict || answer["error"] != "active" {
		t.Errorf("while one is being made: %d %v", status, answer)
	}
}

func TestAnExportIsItsOwnersAlone(t *testing.T) {
	f := newFixture(t)
	ready := time.Now().UTC()
	f.exports.rows["0b8f6a52-1f0e-4a7e-9a52-0a6f6f1f0e4a"] = &repositories.Export{
		ID: "0b8f6a52-1f0e-4a7e-9a52-0a6f6f1f0e4a", PersonID: 5, Status: repositories.StatusReady,
		PackageKey: "exports/5/x.zip", SizeBytes: 99, CreatedAt: ready, ReadyAt: &ready,
	}
	id := "0b8f6a52-1f0e-4a7e-9a52-0a6f6f1f0e4a"

	status, answer := f.call(f.handler.Read, http.MethodGet, 5, id)
	if status != http.StatusOK || answer["download_url"] == nil {
		t.Fatalf("its owner: %d %v", status, answer)
	}

	// Anybody else is told there is none, to read or to delete.
	for name, handler := range map[string]echo.HandlerFunc{"read": f.handler.Read, "delete": f.handler.Delete} {
		if status, answer := f.call(handler, http.MethodGet, 6, id); status != http.StatusNotFound || len(answer) != 0 {
			t.Errorf("%s by somebody else: %d %v", name, status, answer)
		}
	}
	if status, _ := f.call(f.handler.Read, http.MethodGet, 5, "not-an-id"); status != http.StatusNotFound {
		t.Errorf("what is not an id: %d", status)
	}
	if len(f.files.deleted) != 0 {
		t.Fatalf("somebody else's request deleted %v", f.files.deleted)
	}

	// Its owner deletes it: the archive goes, and nothing is offered to fetch.
	if status, _ := f.call(f.handler.Delete, http.MethodDelete, 5, id); status != http.StatusNoContent {
		t.Fatalf("its owner deleting: %d", status)
	}
	if len(f.files.deleted) != 1 || f.files.deleted[0] != "exports/5/x.zip" {
		t.Errorf("deleted %v", f.files.deleted)
	}
	if _, answer := f.call(f.handler.Read, http.MethodGet, 5, id); answer["download_url"] != nil {
		t.Errorf("a deleted export is still offered: %v", answer)
	}

	// The list is the viewer's own.
	if _, answer := f.call(f.handler.Browse, http.MethodGet, 6, ""); len(answer["data"].([]any)) != 0 {
		t.Errorf("somebody else is listed another's exports: %v", answer)
	}
}
