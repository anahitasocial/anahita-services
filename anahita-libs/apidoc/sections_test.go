package apidoc

import (
	"net/http"
	"testing"

	"anahita-libs/apidoc/internal/fixture"

	"github.com/labstack/echo/v4"
)

// A service that serves two kinds of thing, each a section of its own, with
// a route every kind has, explained once in a section of its own.
func kinds(e *echo.Echo) *Service {
	s := NewService("thing-service", "Things", "Things, for the tests.").
		Section("all-things", "Things", "What every thing has.").
		Section("gadgets", "Gadgets", "What only a gadget has.")
	api := s.On(e)
	api.In("all-things").GET("/things/:alias", nothing).
		Summary("Any thing").
		Viewer(Anyone).
		Takes(&fixture.ReadRequest{}).
		Sends(http.StatusOK, fixture.Thing{})
	gadgets := api.In("gadgets").Group("gadgets")
	gadgets.GET("/:alias", nothing).
		Summary("One gadget").
		Viewer(Anyone).
		Takes(&fixture.ReadRequest{}).
		Sends(http.StatusOK, fixture.Gadget{})
	gadgets.DELETE("/:id", nothing).
		Summary("Delete a gadget").
		Viewer(Admin).
		Takes(&fixture.DeleteRequest{}).
		SendsNothing(http.StatusOK, "it is gone").
		Shared("all-things")
	return s
}

func TestSectionsAreTagsOfTheirOwn(t *testing.T) {
	doc := document(t, kinds(echo.New()))

	if got := jsonOf(at(t, doc, "paths", "/things/{alias}", "get", "tags")); got != `["all-things"]` {
		t.Errorf("any thing's tags: %s", got)
	}
	if got := jsonOf(at(t, doc, "paths", "/gadgets/{alias}", "get", "tags")); got != `["gadgets"]` {
		t.Errorf("a gadget's tags: %s", got)
	}
	tags := doc["tags"].([]any)
	if len(tags) != 2 {
		t.Fatalf("tags: %s, want the two sections and not the service", jsonOf(tags))
	}
	gadget := tags[1].(map[string]any)
	if gadget["name"] != "gadgets" || gadget["description"] != "Gadgets" ||
		gadget["x-service"] != "thing-service" || gadget["x-about"] != "What only a gadget has." {
		t.Errorf("the gadgets' tag: %s", jsonOf(gadget))
	}
}

func TestASharedRouteSaysWhereItIsExplained(t *testing.T) {
	doc := document(t, kinds(echo.New()))

	if got := at(t, doc, "paths", "/gadgets/{alias}", "delete", "x-shared"); got != "all-things" {
		t.Errorf("x-shared: %v", got)
	}
	op := at(t, doc, "paths", "/gadgets/{alias}", "get").(map[string]any)
	if _, ok := op["x-shared"]; ok {
		t.Error("a gadget's own route is marked shared")
	}
}

func TestASectionOrASharingThatIsNotThereIsAnError(t *testing.T) {
	s := NewService("thing-service", "Things", "Things.").Section("gadgets", "Gadgets", "")
	api := s.On(echo.New())
	api.In("widgets").GET("/widgets/:alias", nothing).
		Summary("One widget").Viewer(Anyone).Takes(&fixture.ReadRequest{}).Sends(http.StatusOK, fixture.Thing{})
	if _, err := s.Document("."); err == nil {
		t.Error("a route in no such section was described")
	}

	s = NewService("thing-service", "Things", "Things.").Section("gadgets", "Gadgets", "")
	api = s.On(echo.New())
	api.In("gadgets").GET("/gadgets/:alias", nothing).
		Summary("One gadget").Viewer(Anyone).Takes(&fixture.ReadRequest{}).Sends(http.StatusOK, fixture.Thing{}).
		Shared("everything")
	if _, err := s.Document("."); err == nil {
		t.Error("a route shared with no such section was described")
	}
}

func TestAShapeLeavesOutWhatItSays(t *testing.T) {
	doc := document(t, kinds(echo.New()))

	gadget := at(t, doc, "components", "schemas", "Gadget", "properties").(map[string]any)
	for _, gone := range []string{"tags", "meta"} {
		if _, ok := gadget[gone]; ok {
			t.Errorf("a gadget has %q", gone)
		}
	}
	if _, ok := gadget["name"]; !ok {
		t.Error("a gadget lost what it should keep")
	}
	if got := jsonOf(at(t, doc, "components", "schemas", "Gadget", "required")); got == "" || contains(got, `"tags"`) {
		t.Errorf("a gadget's required fields: %s", got)
	}
	thing := at(t, doc, "components", "schemas", "Thing", "properties").(map[string]any)
	if _, ok := thing["tags"]; !ok {
		t.Error("the thing a gadget is made of lost its tags")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
