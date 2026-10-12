package worker

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"

	"export/archive"
	"export/data/repositories"
)

// The worker, with the graph and storage stood in for. What goes into an
// archive is tested in package archive; what state an export may go to, in
// the repository's tests against MySQL.

type fakeExports struct {
	repositories.ExportsInterface
	export   *repositories.Export
	released int
}

func (f *fakeExports) Claim() (*repositories.Export, error) {
	if f.export.Status != repositories.StatusPending {
		return nil, nil
	}
	f.export.Status = repositories.StatusBuilding
	f.export.Attempts++
	copied := *f.export
	return &copied, nil
}

func (f *fakeExports) Read(id string) (*repositories.Export, error) {
	copied := *f.export
	return &copied, nil
}

func (f *fakeExports) ToReady(id, key string, size int64, keepFor time.Duration) error {
	if f.export.Status != repositories.StatusBuilding {
		return repositories.ErrNotInState
	}
	f.export.Status, f.export.PackageKey, f.export.SizeBytes = repositories.StatusReady, key, size
	return nil
}

func (f *fakeExports) ToFailed(id, reason string) error {
	f.export.Status, f.export.Error = repositories.StatusFailed, reason
	return nil
}

func (f *fakeExports) Release(id string) error {
	f.released++
	f.export.Status = repositories.StatusPending
	return nil
}

type source struct {
	// Called as the posts are read.
	during func()
}

func (s *source) Profile(id int64) (*entities.AnahitaNode, error) {
	person := entities.NewAnahitaNode()
	person.ID, person.Type, person.Name, person.Alias = id, constants.NodeActorPersonEntityType, "Ana", "ana"
	return person, nil
}

func (s *source) Authored(id, after int64, limit uint64) ([]*entities.AnahitaNode, error) {
	if s.during != nil {
		s.during()
	}
	if after > 0 {
		return nil, nil
	}
	note := entities.NewAnahitaNode()
	note.ID, note.Type, note.Body, note.CreatedBy = 10, "node.medium.text-service.note.v1", "The heap is warm", id
	return []*entities.AnahitaNode{note}, nil
}

func (s *source) PhotoFiles(ids []int64) ([]*entities.PhotoFile, error) { return nil, nil }
func (s *source) Connected(id int64, kind string, offset uint64) ([]*entities.AnahitaNode, error) {
	return nil, nil
}
func (s *source) Saved(id int64, offset uint64) ([]*entities.AnahitaNode, error) { return nil, nil }

type noFiles struct{}

func (noFiles) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, archive.ErrNoFile
}

// store is storage, behind addresses like the real ones.
type store struct {
	services.FileTransfersInterface
	mu     sync.Mutex
	kept   map[string][]byte
	types  map[string]string
	server *httptest.Server
	down   bool
}

func newStore(t *testing.T) *store {
	s := &store{kept: map[string][]byte{}, types: map[string]string{}}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.down {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/")
		body, _ := io.ReadAll(r.Body)
		s.kept[key] = body
		s.types[key] = r.Header.Get("Content-Type")
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *store) StoreURL(path, contentType string) (string, error) {
	return s.server.URL + "/" + path, nil
}

func (s *store) Delete(req *services.FileDeleteRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.kept, req.Path)
	return nil
}

func newWorker(t *testing.T) (*Worker, *fakeExports, *store, *source) {
	exports := &fakeExports{export: &repositories.Export{
		ID: "0b8f6a52-1f0e-4a7e-9a52-0a6f6f1f0e4a", PersonID: 5, Status: repositories.StatusPending,
	}}
	storage := newStore(t)
	src := &source{}
	return &Worker{
		Exports:   exports,
		Source:    src,
		Files:     noFiles{},
		Transfers: storage,
		Remover:   storage,
		WebURL:    "https://www.example.org",
		SiteName:  "The Allotment",
		Dir:       t.TempDir(),
		KeepFor:   7 * 24 * time.Hour,
		Timeout:   time.Minute,
	}, exports, storage, src
}

func TestARequestBecomesAnArchiveInStorage(t *testing.T) {
	w, exports, storage, _ := newWorker(t)

	if worked, err := w.Once(); err != nil || !worked {
		t.Fatalf("worked %v %v", worked, err)
	}

	key := "assets/v1/export-service/5/0b8f6a52-1f0e-4a7e-9a52-0a6f6f1f0e4a.zip"
	if exports.export.Status != repositories.StatusReady || exports.export.PackageKey != key {
		t.Fatalf("ended as %+v", exports.export)
	}
	stored := storage.kept[key]
	if int64(len(stored)) != exports.export.SizeBytes || storage.types[key] != ArchiveType {
		t.Fatalf("stored %d bytes as %q, recorded %d", len(stored), storage.types[key], exports.export.SizeBytes)
	}

	// What was stored is the archive, whole.
	reader, err := zip.NewReader(bytes.NewReader(stored), int64(len(stored)))
	if err != nil {
		t.Fatalf("what was stored is not a zip: %v", err)
	}
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	if !names["index.html"] || !names["data/posts.ndjson"] {
		t.Errorf("the archive holds %v", names)
	}

	if worked, _ := w.Once(); worked {
		t.Errorf("worked again")
	}
}

func TestAnExportGivenUpWhileItIsMadeIsNotKept(t *testing.T) {
	w, exports, storage, src := newWorker(t)
	src.during = func() { exports.export.Status = repositories.StatusCancelled }

	w.Once()

	if exports.export.Status != repositories.StatusCancelled || len(storage.kept) != 0 || exports.released != 0 {
		t.Errorf("ended as %+v, kept %d, let go %d", exports.export, len(storage.kept), exports.released)
	}
}

func TestATroubleThatMayPassIsTriedAgainAndThenGivenUp(t *testing.T) {
	w, exports, storage, _ := newWorker(t)
	storage.down = true

	for attempt := 1; attempt <= repositories.MaxAttempts; attempt++ {
		if worked, _ := w.Once(); !worked {
			t.Fatalf("attempt %d: nothing taken", attempt)
		}
	}
	if exports.export.Status != repositories.StatusFailed || exports.released != repositories.MaxAttempts-1 || len(storage.kept) != 0 {
		t.Errorf("ended as %+v, let go %d", exports.export, exports.released)
	}
}
