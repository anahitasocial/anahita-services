package handlers

import (
	"anahita-libs/apidoc"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRequestLanguage(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		// Not saying is allowed, and nothing is guessed in its place.
		{"", "", true},
		{"fr", "fr", true},
		{"FR-ca", "fr-CA", true},
		{"und", "und", true},
		{"french", "", false},
		{"xx", "", false},
	} {
		got, ok := RequestLanguage(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("RequestLanguage(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestARefusedLanguageIsABadRequest(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodPost, "/notes/5/", nil), rec)

	if err := RefuseLanguage(c); err != nil {
		t.Fatal(err)
	}
	// As posting a note answers it.
	apidoc.Conforms(t, "../../../../docs/api/text-service.json", "POST /notes/5/", rec)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), ErrLanguageInvalid) {
		t.Errorf("answered %d %s", rec.Code, rec.Body.String())
	}
}

// Every handler that creates or edits a post reads the language the same
// way: through RequestLanguage, so a tag is checked and spelled the usual way
// before it is saved, and a bad one is refused. Read out of the handlers, as
// the audience check is.
func TestEveryPostHandlerReadsTheLanguage(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")

	for path, alias := range map[string]string{
		"text-service/cmd/api/handlers/note.go":    "an_handlers",
		"text-service/cmd/api/handlers/article.go": "an_handlers",
		"text-service/cmd/api/handlers/topic.go":   "an_handlers",
		"photo-service/cmd/api/handlers/photo.go":  "handlers",
	} {
		source, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		// Once when a post is created and once when it is edited.
		if got := strings.Count(string(source), alias+".RequestLanguage(req.Language)"); got != 2 {
			t.Errorf("%s reads the language in %d place(s), want 2: creating and editing", path, got)
		}
		if strings.Contains(string(source), "medium.Language = req.Language") {
			t.Errorf("%s saves a language without checking it", path)
		}
	}
}
