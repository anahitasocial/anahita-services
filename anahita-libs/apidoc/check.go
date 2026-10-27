package apidoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
)

// The checks that keep the reference true, run in `make test`:
//
//  1. Every route registered with echo is described, and every description
//     is of a route.
//  2. The document committed is the one the code makes now.
//
// `make api_docs` sets UpdateEnv, which writes the documents instead of
// comparing them.

// UpdateEnv, set to anything, has Check write the reference rather than
// compare it.
const UpdateEnv = "APIDOC_UPDATE"

// T is the part of *testing.T a check needs.
type T interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Check makes the service's document and compares it with `file`, after
// comparing the routes echo has with those described. Run from a test in
// the package that registers the routes.
func Check(t T, e *echo.Echo, s *Service, file string) {
	t.Helper()

	registered := map[string]bool{}
	for _, r := range e.Routes() {
		if r.Method == echo.RouteNotFound {
			continue
		}
		registered[r.Method+" "+normalizePath(r.Path)] = true
	}
	described := map[string]bool{}
	for _, r := range s.routes {
		described[r.Method+" "+r.Path] = true
	}
	for _, route := range sortedKeys(registered) {
		if !described[route] {
			t.Errorf("%s is registered and not described: register it through apidoc, with what it takes and sends", route)
		}
	}
	for _, route := range sortedKeys(described) {
		if !registered[route] {
			t.Errorf("%s is described and not registered", route)
		}
	}

	doc, err := s.Document(".")
	if err != nil {
		t.Fatalf("describing %s: %v", s.Name, err)
	}
	compare(t, file, doc)
}

// compare writes `doc` to `file` under UpdateEnv, and otherwise fails when
// they differ.
func compare(t T, file string, doc []byte) {
	t.Helper()
	if os.Getenv(UpdateEnv) != "" {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatalf("%v", err)
		}
		if err := os.WriteFile(file, doc, 0o644); err != nil {
			t.Fatalf("%v", err)
		}
		return
	}
	committed, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("reading %s: %v; run `make api_docs` and commit what it writes", file, err)
	}
	if !bytes.Equal(committed, doc) {
		t.Errorf("%s is not what the code describes now; run `make api_docs` and commit what it writes", file)
	}
}

// CheckCombined makes the one document of every service from the files in
// `dir` and compares it with `file`, which is left out of what is combined.
func CheckCombined(t T, dir, file string) {
	t.Helper()
	doc, err := Combine(dir, filepath.Base(file))
	if err != nil {
		t.Fatalf("combining the reference: %v", err)
	}
	compare(t, file, doc)
}

// Combine is one document of every service's in `dir`, leaving out `skip`.
// A path or a method described by two services, or one component name for
// two shapes, is an error.
func Combine(dir, skip string) ([]byte, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	paths := map[string]any{}
	schemas := map[string]any{}
	var tags []any
	for _, file := range files {
		if filepath.Base(file) == skip {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var doc struct {
			Tags       []any                     `json:"tags"`
			Paths      map[string]map[string]any `json:"paths"`
			Components struct {
				Schemas map[string]any `json:"schemas"`
			} `json:"components"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		tags = append(tags, doc.Tags...)
		for path, item := range doc.Paths {
			have, _ := paths[path].(map[string]any)
			if have == nil {
				have = map[string]any{}
				paths[path] = have
			}
			for method, op := range item {
				if _, taken := have[method]; taken {
					return nil, fmt.Errorf("%s %s is described by two services", strings.ToUpper(method), path)
				}
				have[method] = op
			}
		}
		for name, schema := range doc.Components.Schemas {
			if have, ok := schemas[name]; ok {
				a, _ := json.Marshal(have)
				b, _ := json.Marshal(schema)
				if !bytes.Equal(a, b) {
					return nil, fmt.Errorf("%s: %s is described differently by another service; rebuild every service's reference", file, name)
				}
				continue
			}
			schemas[name] = schema
		}
	}

	// The services in the order of their groups, and the groups as
	// x-tagGroups, which readers of OpenAPI (Redoc, Scalar) show as headings.
	at := places()
	for _, tag := range tags {
		name, _ := tag.(map[string]any)["name"].(string)
		if _, ok := at[name]; !ok {
			return nil, fmt.Errorf("%s is in none of the reference's groups; put it in one (anahita-libs/apidoc/groups.go)", name)
		}
	}
	sort.SliceStable(tags, func(i, j int) bool {
		a, _ := tags[i].(map[string]any)["name"].(string)
		b, _ := tags[j].(map[string]any)["name"].(string)
		return at[a] < at[b]
	})
	var groups []any
	for _, g := range Groups {
		var names []any
		for _, name := range g.Services {
			for _, tag := range tags {
				if tag.(map[string]any)["name"] == name {
					names = append(names, name)
				}
			}
		}
		if len(names) > 0 {
			groups = append(groups, map[string]any{"name": g.Name, "tags": names})
		}
	}

	return encode(map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "Anahita",
			"description": "Every route of every Anahita service. Each service's own is beside this file. What is true of all of them is in docs/api.md.",
			"version":     "dev",
			"license":     license,
		},
		"servers":     servers,
		"tags":        tags,
		"x-tagGroups": groups,
		"paths":       paths,
		"components": map[string]any{
			"schemas":         schemas,
			"securitySchemes": securitySchemes,
		},
	})
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
