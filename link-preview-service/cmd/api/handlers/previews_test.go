package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"linkpreview/data/services"
	"linkpreview/fetch"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	an_services "anahita-libs/data/services"
	"anahita-libs/linkpreviews"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// The endpoints, over a fetcher and a storage that are held in memory. What
// the real fetcher will and will not connect to is tested in package fetch.

type fakeFetcher struct {
	pages     int
	pictures  int
	page      *fetch.Page
	pageErr   error
	thumbErr  error
	lastAsked string
}

func (f *fakeFetcher) Page(address string) (*fetch.Page, string, error) {
	f.pages++
	f.lastAsked = address
	if f.pageErr != nil {
		return nil, "", f.pageErr
	}
	return f.page, address, nil
}

func (f *fakeFetcher) Thumbnail(address string) ([]byte, string, string, error) {
	f.pictures++
	if f.thumbErr != nil {
		return nil, "", "", f.thumbErr
	}
	return []byte("small picture"), strings.Repeat("a", 64) + ".jpg", "image/jpeg", nil
}

type fakeFiles struct {
	an_services.FilesGRPCInterface
	stored map[string][]byte
}

func (f *fakeFiles) Exists(req *an_services.FileExistsRequest) (bool, error) {
	_, ok := f.stored[req.Path]
	return ok, nil
}

func (f *fakeFiles) Add(req *an_services.FileAddRequest) (*entities.File, error) {
	f.stored[req.Path] = req.Content
	return &entities.File{}, nil
}

func (f *fakeFiles) Presign(req *an_services.FilePresignRequest) (string, error) {
	if _, ok := f.stored[req.Path]; !ok {
		return "", errors.New("no such file")
	}
	return "https://storage.example/" + req.Path + "?signed", nil
}

type memory struct {
	kept   map[string]string
	counts map[string]int64
}

func (m *memory) Get(key string) (string, bool, error) {
	value, ok := m.kept[key]
	return value, ok, nil
}

func (m *memory) Set(key, value string, ttl time.Duration) error {
	m.kept[key] = value
	return nil
}

func (m *memory) Incr(key string, ttl time.Duration) (int64, error) {
	m.counts[key]++
	return m.counts[key], nil
}

func (m *memory) Del(keys ...string) error { return nil }

type fixture struct {
	t       *testing.T
	echo    *echo.Echo
	handler *previewsHandler
	fetcher *fakeFetcher
	files   *fakeFiles
	memory  *memory
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	f := &fixture{
		t:    t,
		echo: echo.New(),
		fetcher: &fakeFetcher{page: &fetch.Page{
			Title:       "Compost & you",
			Description: "How to make it",
			SiteName:    "The Allotment",
			Image:       "https://example.org/heap.jpg",
		}},
		files:  &fakeFiles{stored: map[string][]byte{}},
		memory: &memory{kept: map[string]string{}, counts: map[string]int64{}},
	}
	f.echo.Validator = validators.NewRequestValidator(validator.New())
	f.handler = NewHandlers(&Env{
		Srv:     &services.Services{FilesGRPC: f.files},
		Fetcher: f.fetcher,
		Store:   linkpreviews.NewStore(f.memory),
		Limiter: f.memory,
	}).Previews

	return f
}

func person(id int64) *entities.Viewer {
	viewer := entities.NewViewer()
	if id > 0 {
		viewer.ID = id
		viewer.Authenticated = true
	}
	return viewer
}

func (f *fixture) read(viewer *entities.Viewer, query string) (int, map[string]any) {
	f.t.Helper()

	rec := httptest.NewRecorder()
	c := f.echo.NewContext(httptest.NewRequest(http.MethodGet, "/?"+query, nil), rec)
	c.Set(constants.ViewerContextKey, viewer)
	if err := f.handler.Read(c); err != nil {
		f.t.Fatalf("handler: %v", err)
	}
	answers.Handler(f.handler.Read, rec)

	answer := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &answer)
	}
	return rec.Code, answer
}

func TestAPreviewIsFetchedOnceAndKept(t *testing.T) {
	f := newFixture(t)

	status, answer := f.read(person(1), "url=https://Example.org/post%23top")
	if status != http.StatusOK {
		t.Fatalf("answered %d %v", status, answer)
	}
	if answer["title"] != "Compost & you" || answer["site_name"] != "The Allotment" || answer["url"] != "https://example.org/post" {
		t.Errorf("sent %v", answer)
	}
	// The picture is this installation's copy, by a path under the API.
	if image := answer["image"].(string); !strings.HasPrefix(image, linkpreviews.ImagePath) || strings.Contains(image, "example.org") {
		t.Errorf("picture sent as %q", image)
	}
	if len(f.files.stored) != 1 {
		t.Errorf("%d pictures stored", len(f.files.stored))
	}

	// Asked for again, by somebody else: not fetched again.
	f.read(person(2), "url=https://example.org/post")
	if f.fetcher.pages != 1 || f.fetcher.pictures != 1 {
		t.Errorf("fetched %d pages and %d pictures for one link", f.fetcher.pages, f.fetcher.pictures)
	}
}

func TestWhoIsAnsweredAndWithWhat(t *testing.T) {
	f := newFixture(t)

	if status, _ := f.read(person(0), "url=https://example.org/"); status != http.StatusUnauthorized {
		t.Errorf("a visitor answered %d", status)
	}
	for _, query := range []string{"", "url=", "url=ftp://example.org/x", "url=javascript:alert(1)", "url=example.org"} {
		if status, _ := f.read(person(1), query); status != http.StatusBadRequest {
			t.Errorf("%q answered %d", query, status)
		}
	}
	if f.fetcher.pages != 0 {
		t.Errorf("a page was fetched for a visitor or for what is not an address")
	}
}

func TestAPageThatCannotBeReadIsNothingToShowAndIsNotTriedAgainAtOnce(t *testing.T) {
	f := newFixture(t)
	f.fetcher.pageErr = fetch.ErrNotAllowed

	for i := 0; i < 3; i++ {
		if status, _ := f.read(person(1), "url=http://10.0.0.5/admin"); status != http.StatusNoContent {
			t.Fatalf("answered %d", status)
		}
	}
	if f.fetcher.pages != 1 {
		t.Errorf("a page that failed was tried %d times", f.fetcher.pages)
	}

	// A page that says nothing of itself is nothing to show either.
	f.fetcher.pageErr = nil
	f.fetcher.page = &fetch.Page{}
	if status, _ := f.read(person(1), "url=https://example.org/empty"); status != http.StatusNoContent {
		t.Errorf("a page with no title answered %d", status)
	}
}

func TestAPreviewWithoutItsPictureIsStillShown(t *testing.T) {
	f := newFixture(t)
	f.fetcher.thumbErr = errors.New("not a picture")

	status, answer := f.read(person(1), "url=https://example.org/post")
	if status != http.StatusOK || answer["title"] != "Compost & you" {
		t.Fatalf("answered %d %v", status, answer)
	}
	if _, has := answer["image"]; has || len(f.files.stored) != 0 {
		t.Errorf("a picture was sent or stored: %v", answer)
	}
}

func TestAskingIsLimited(t *testing.T) {
	f := newFixture(t)

	var last int
	for i := 0; i <= int(previewLimit.Max); i++ {
		last, _ = f.read(person(1), "url=https://example.org/post")
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("request %d answered %d", previewLimit.Max+1, last)
	}
	// Somebody else is not held to it.
	if status, _ := f.read(person(2), "url=https://example.org/post"); status != http.StatusOK {
		t.Errorf("somebody else answered %d", status)
	}
}

func TestAPictureIsReachedOnlyByItsOwnName(t *testing.T) {
	f := newFixture(t)
	f.read(person(1), "url=https://example.org/post")
	name := strings.Repeat("a", 64) + ".jpg"

	image := func(name string) (int, string) {
		rec := httptest.NewRecorder()
		c := f.echo.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
		c.SetParamNames("name")
		c.SetParamValues(name)
		if err := f.handler.Image(c); err != nil {
			t.Fatalf("handler: %v", err)
		}
		answers.Handler(f.handler.Image, rec)
		return rec.Code, rec.Header().Get("Location")
	}

	status, location := image(name)
	if status != http.StatusFound || !strings.Contains(location, linkpreviews.StoragePrefix+name) {
		t.Errorf("answered %d, to %q", status, location)
	}

	for _, bad := range []string{
		"../avatars/someone.jpg", "..%2Favatars%2Fx.jpg", "x.jpg", strings.Repeat("a", 64) + ".svg",
		strings.Repeat("A", 64) + ".jpg", strings.Repeat("b", 64) + ".jpg", "",
	} {
		if status, location := image(bad); status != http.StatusNotFound || location != "" {
			t.Errorf("%q answered %d, to %q", bad, status, location)
		}
	}
}
