package apidoc

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const conformsReference = `{
  "paths": {
    "/notes/{id}": {
      "get": {
        "responses": {
          "200": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Note"}}}},
          "404": {"description": "there is no such note"}
        }
      }
    }
  },
  "components": {"schemas": {
    "Note": {
      "type": "object",
      "required": ["id", "kind"],
      "properties": {
        "id": {"type": "integer"},
        "kind": {"type": "string", "enum": ["note", "article"]},
        "tags": {"type": "array", "items": {"type": "string"}},
        "author": {"type": "object", "properties": {"alias": {"type": "string"}}},
        "meta": {"type": "object", "additionalProperties": {"type": "boolean"}}
      }
    }
  }}
}`

// recordingT keeps what a check says instead of failing the test.
type recordingT struct{ errors []string }

func (r *recordingT) Helper() {}
func (r *recordingT) Errorf(f string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(f, args...))
}
func (r *recordingT) Fatalf(f string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(f, args...))
}

func TestConforms(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notes.json")
	if err := os.WriteFile(file, []byte(conformsReference), 0o644); err != nil {
		t.Fatal(err)
	}

	answer := func(status int, contentType, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		if contentType != "" {
			rec.Header().Set("Content-Type", contentType)
		}
		rec.WriteHeader(status)
		rec.WriteString(body)
		return rec
	}
	json := "application/json; charset=UTF-8"

	cases := []struct {
		name  string
		route string
		rec   *httptest.ResponseRecorder
		want  string // in what is said, or "" for nothing wrong
	}{
		{"an answer as described", "GET /notes/:id", answer(200, json, `{"id": 7, "kind": "note", "tags": ["a"], "author": {"alias": "ana"}, "meta": {"x": true}}`), ""},
		{"null where Go writes nil", "GET /notes/{id}", answer(200, json, `{"id": 7, "kind": "note", "tags": null}`), ""},
		{"a refusal as described", "GET /notes/:id", answer(404, "", ""), ""},
		{"a server failure", "GET /notes/:id", answer(500, "", "oops"), ""},
		{"a status not listed", "GET /notes/:id", answer(409, "", ""), "and not this"},
		{"a body where there is none", "GET /notes/:id", answer(404, json, `{"error": "gone"}`), "nothing in it"},
		{"a field always there, missing", "GET /notes/:id", answer(200, json, `{"id": 7}`), "is missing"},
		{"a field the reference does not name", "GET /notes/:id", answer(200, json, `{"id": 7, "kind": "note", "Kind": "x"}`), "not in the reference"},
		{"a value of the wrong type", "GET /notes/:id", answer(200, json, `{"id": "7", "kind": "note"}`), "not a number"},
		{"a value not one it may be", "GET /notes/:id", answer(200, json, `{"id": 7, "kind": "poem"}`), "is not one of"},
		{"a value in a map of the wrong type", "GET /notes/:id", answer(200, json, `{"id": 7, "kind": "note", "meta": {"x": 1}}`), "not true or false"},
		{"another type of body", "GET /notes/:id", answer(200, "text/html", `<p>`), "the reference says"},
		{"a path sent to", "GET /notes/7?x=1", answer(404, "", ""), ""},
		{"a route not described", "DELETE /notes/:id", answer(200, "", ""), "no such route"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := &recordingT{}
			Conforms(got, file, c.route, c.rec)
			said := strings.Join(got.errors, "\n")
			if c.want == "" && said != "" {
				t.Errorf("found something wrong: %s", said)
			}
			if c.want != "" && !strings.Contains(said, c.want) {
				t.Errorf("said %q, want it to say %q", said, c.want)
			}
		})
	}
}

func TestAnswersAreCheckedTogether(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notes.json")
	if err := os.WriteFile(file, []byte(conformsReference), 0o644); err != nil {
		t.Fatal(err)
	}
	answers := NewAnswers(file)

	good := httptest.NewRecorder()
	good.WriteHeader(404)
	answers.Route("GET /notes/7", good)

	bad := httptest.NewRecorder()
	bad.Header().Set("Content-Type", "application/json")
	bad.WriteHeader(200)
	bad.WriteString(`{"id": 7}`)
	answers.Route("GET /notes/7", bad)
	answers.Route("GET /notes/8", bad)
	// Read away, as a test decoding it would.
	bad.Body.Reset()

	problems := answers.Problems()
	if len(problems) != 1 || !strings.Contains(problems[0], "`kind` is missing") {
		t.Errorf("problems: %q, want the one missing field, said once", problems)
	}
}
