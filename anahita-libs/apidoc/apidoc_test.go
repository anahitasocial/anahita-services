package apidoc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"anahita-libs/apidoc/internal/fixture"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

func nothing(c echo.Context) error { return nil }

func pass(next echo.HandlerFunc) echo.HandlerFunc { return next }

// things is a small service, described as a real one is.
func things(e *echo.Echo) *Service {
	s := NewService("thing-service", "Things", "Things, for the tests.")
	api := s.On(e)
	g := api.Group("things", pass)
	g.GET("/", nothing).
		Summary("Things").
		Viewer(Anyone).
		Takes(&fixture.BrowseRequest{Paging: fixture.Paging{Limit: 20}, Sort: "recent"}).
		Sends(http.StatusOK, responses.List[*fixture.Thing]{})
	g.GET("/:alias", nothing).
		Summary("One thing").
		Viewer(Anyone).
		Takes(&fixture.ReadRequest{}).
		Sends(http.StatusOK, fixture.Thing{}).
		Refuses(http.StatusNotFound, "there is no such thing").
		Refuses(http.StatusNotFound, "or the viewer may not see it")
	g.DELETE("/:id", nothing).
		Summary("Delete a thing").
		Viewer(Admin).
		Scope("things:write").
		Takes(&fixture.DeleteRequest{}).
		SendsNothing(http.StatusOK, "it is gone")
	g.POST("/", nothing).
		Summary("Make a thing").
		Viewer(SignedIn).
		StepUp().
		Takes(&fixture.AddRequest{}).
		Sends(http.StatusCreated, fixture.Thing{})
	g.PATCH("/:alias", nothing).
		Summary("Change a thing").
		Viewer(Admin).
		Takes(&fixture.ReadRequest{}).
		NotBuilt("things cannot be changed")
	return s
}

func document(t *testing.T, s *Service) map[string]any {
	t.Helper()
	raw, err := s.Document(".")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// at walks a document: at(doc, "paths", "/things/{alias}", "get").
func at(t *testing.T, v any, keys ...string) any {
	t.Helper()
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("no %q in %v", k, v)
		}
		v, ok = m[k]
		if !ok {
			t.Fatalf("no %q among %v", k, keysOf(m))
		}
	}
	return v
}

func keysOf(m map[string]any) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func jsonOf(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func TestARouteIsDescribedFromWhatItTakesAndSends(t *testing.T) {
	doc := document(t, things(echo.New()))

	// The path as OpenAPI writes it, one name for one place: DELETE's
	// :id is called by the name GET gave that place.
	read := at(t, doc, "paths", "/things/{alias}", "get")
	del := at(t, doc, "paths", "/things/{alias}", "delete")
	if got := at(t, read, "operationId"); got != "getThingsByAlias" {
		t.Errorf("operationId %v", got)
	}

	// A path value: required, described by its field's comment, with what
	// it may be; a rule JSON Schema has no word for is kept as it is.
	params := jsonOf(at(t, read, "parameters"))
	for _, want := range []string{`"name":"alias"`, `"in":"path"`, `"required":true`, `"description":"Its name."`, `"maxLength":100`, `"x-validate":"custom_rule"`} {
		if !strings.Contains(params, want) {
			t.Errorf("the path value lacks %s: %s", want, params)
		}
	}
	// A value of another route's path is not one of this route's.
	if strings.Contains(params, "owner_id") {
		t.Errorf("owner_id is described: %s", params)
	}
	if got := jsonOf(at(t, del, "parameters")); !strings.Contains(got, `"name":"alias"`) || !strings.Contains(got, "\"description\":\"Here, the `id`. Its number.\"") {
		t.Errorf("DELETE's path value: %s", got)
	}

	// Query values, from an embedded struct too, with defaults from the
	// value the handler starts from and choices from oneof. A field with
	// no tag is not read from a request without a body.
	browse := jsonOf(at(t, doc, "paths", "/things/", "get", "parameters"))
	for _, want := range []string{`"name":"start"`, `"name":"limit"`, `"default":20`, `"maximum":50`, `"enum":["recent","popular"]`, `"default":"recent"`} {
		if !strings.Contains(browse, want) {
			t.Errorf("browsing lacks %s: %s", want, browse)
		}
	}
	if strings.Contains(browse, "Internal") {
		t.Errorf("an untagged field is a parameter: %s", browse)
	}

	// A list: written out where it is used, of things by reference.
	list := jsonOf(at(t, doc, "paths", "/things/", "get", "responses", "200", "content", "application/json", "schema"))
	for _, want := range []string{`"data"`, `"$ref":"#/components/schemas/Thing"`, `"$ref":"#/components/schemas/Pagination"`, `"null"`} {
		if !strings.Contains(list, want) {
			t.Errorf("the list lacks %s: %s", want, list)
		}
	}

	// A thing, described once: embedded fields as its own, what may be
	// left out not required, what may be null said so, a field of its own
	// kind by reference, and nothing that JSON leaves out.
	thing := jsonOf(at(t, doc, "components", "schemas", "Thing"))
	for _, want := range []string{`"id"`, `"description":"Thing is something there is."`, `"format":"date-time"`, `"required":["id","created_at","tags"]`, `"type":["array","null"]`, `"parent":{"$ref":"#/components/schemas/Thing","description":"The thing it belongs to."}`} {
		if !strings.Contains(thing, want) {
			t.Errorf("the thing lacks %s: %s", want, thing)
		}
	}
	for _, unwanted := range []string{"Secret", "hidden", `"-"`} {
		if strings.Contains(thing, unwanted) {
			t.Errorf("the thing has %s: %s", unwanted, thing)
		}
	}

	// Refusals: several reasons for one status are one answer; and what
	// every route of its kind may refuse is added.
	if got := at(t, read, "responses", "404", "description"); got != "there is no such thing; or the viewer may not see it" {
		t.Errorf("404: %v", got)
	}
	if _, ok := at(t, read, "responses").(map[string]any)["401"]; ok {
		t.Error("a route for anyone refuses 401")
	}
	delResponses := at(t, del, "responses").(map[string]any)
	for _, status := range []string{"200", "400", "401", "403"} {
		if _, ok := delResponses[status]; !ok {
			t.Errorf("DELETE lacks %s: %v", status, keysOf(delResponses))
		}
	}
	if got := jsonOf(at(t, del, "security")); got != `[{"bearer":["things:write"]}]` {
		t.Errorf("security %s", got)
	}
	if got := jsonOf(at(t, read, "security")); got != `[{},{"bearer":[]}]` {
		t.Errorf("a route for anyone: security %s", got)
	}

	// A body: JSON fields and a form field, what is required, bounds.
	add := at(t, doc, "paths", "/things/", "post")
	body := jsonOf(at(t, add, "requestBody"))
	for _, want := range []string{`"application/json"`, `"multipart/form-data"`, `"required":["name"]`, `"minLength":1`, `"maxLength":20`, `"minimum":0`, `"maximum":1`, `"picture"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the body lacks %s: %s", want, body)
		}
	}
	if at(t, add, "x-step-up") != true || !strings.Contains(at(t, add, "description").(string), "proved who they are again") {
		t.Error("making a thing does not say it needs the person to prove who they are again")
	}

	// Not built: it says so, and answers 501 alone.
	patch := at(t, doc, "paths", "/things/{alias}", "patch")
	if got := jsonOf(at(t, patch, "responses")); got != `{"501":{"description":"things cannot be changed"}}` {
		t.Errorf("not built: %s", got)
	}
}

// fakeT records what a check says.
type fakeT struct {
	errors []string
	fatal  string
}

func (f *fakeT) Helper() {}
func (f *fakeT) Errorf(format string, args ...any) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}
func (f *fakeT) Fatalf(format string, args ...any) {
	f.fatal = fmt.Sprintf(format, args...)
	panic(f)
}

func check(e *echo.Echo, s *Service, file string) (f *fakeT) {
	f = &fakeT{}
	defer func() {
		if r := recover(); r != nil && r != f {
			panic(r)
		}
	}()
	Check(f, e, s, file)
	return f
}

// THE RULES THESE GUARD: every route is described, every description is
// of a route, and the file committed is what the code makes now.
func TestTheChecks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "thing-service.json")

	e := echo.New()
	s := things(e)

	// Written under UpdateEnv, and then the same.
	t.Setenv(UpdateEnv, "1")
	if f := check(e, s, file); f.fatal != "" || len(f.errors) > 0 {
		t.Fatalf("writing: %v %v", f.fatal, f.errors)
	}
	t.Setenv(UpdateEnv, "")
	if f := check(e, s, file); f.fatal != "" || len(f.errors) > 0 {
		t.Fatalf("comparing: %v %v", f.fatal, f.errors)
	}

	// Changed by hand, or by the code: told.
	if err := os.WriteFile(file, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if f := check(e, s, file); len(f.errors) != 1 || !strings.Contains(f.errors[0], "make api_docs") {
		t.Errorf("a stale file: %v", f.errors)
	}

	// A route registered past apidoc.
	e.GET("/things/:alias/secret", nothing)
	if f := check(e, s, file); len(f.errors) == 0 || !strings.Contains(strings.Join(f.errors, "\n"), "GET /things/:alias/secret is registered and not described") {
		t.Errorf("an undescribed route: %v", f.errors)
	}

	// A description that lacks what it must say.
	bare := NewService("bare", "Bare", "")
	be := echo.New()
	bare.On(be).GET("/bare", nothing)
	if f := check(be, bare, file); !strings.Contains(f.fatal, "has no a summary, no who may call it (Viewer), no what it sends") {
		t.Errorf("an empty description: %q", f.fatal)
	}

	// A value in the path that nothing describes.
	loose := NewService("loose", "Loose", "")
	le := echo.New()
	loose.On(le).GET("/loose/:id", nothing).Summary("Loose").Viewer(Anyone).Sends(200, nil)
	if f := check(le, loose, file); !strings.Contains(f.fatal, "the path's id not described") {
		t.Errorf("an undescribed path value: %q", f.fatal)
	}
}

func TestCombiningServices(t *testing.T) {
	dir := t.TempDir()
	s := things(echo.New())
	one, err := s.Document(".")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "thing-service.json"), one, 0o644); err != nil {
		t.Fatal(err)
	}

	// A service in none of the groups is an error.
	if _, err := Combine(dir, "anahita.json"); err == nil || !strings.Contains(err.Error(), "none of the reference's groups") {
		t.Errorf("a service in no group: %v", err)
	}
	saved := Groups
	Groups = []Group{{"Things", []string{"thing-service"}}}
	t.Cleanup(func() { Groups = saved })

	combined, err := Combine(dir, "anahita.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(combined, &doc); err != nil {
		t.Fatal(err)
	}
	at(t, doc, "paths", "/things/{alias}", "get")
	at(t, doc, "components", "schemas", "Thing")
	if g := doc["x-tagGroups"].([]any)[0].(map[string]any); g["name"] != "Things" {
		t.Errorf("x-tagGroups: %v", doc["x-tagGroups"])
	}

	// The same route from a second service is an error.
	if err := os.WriteFile(filepath.Join(dir, "other-service.json"), one, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Combine(dir, "anahita.json"); err == nil || !strings.Contains(err.Error(), "described by two services") {
		t.Errorf("two services, one route: %v", err)
	}
}
