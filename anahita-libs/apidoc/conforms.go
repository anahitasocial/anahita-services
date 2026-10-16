package apidoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
)

// The third check: what a handler answers is what the reference says it
// answers. Run from the handler tests there are, after each answer:
//
//	apidoc.Conforms(t, "../../../../docs/api/auth-service.json", "POST /totp", rec)
//
// It fails when the status is not one the route lists, when an answer the
// route sends with nothing in it has something in it, and when a body is
// not of its schema: a field missing that is always there, a value of the
// wrong type or not one of those it may be, or a field the reference does
// not name. That last is what a renamed field looks like.
//
// A 500 is not checked: every route may fail.

// Conforms checks the answer `rec` holds against what the reference in
// `file` says `route` answers. The route is its method and its path as
// echo has it ("PATCH /hashtags/:alias") or as the reference writes it.
func Conforms(t T, file, route string, rec *httptest.ResponseRecorder) {
	t.Helper()
	ref, err := loadReference(file)
	if err != nil {
		t.Fatalf("%v", err)
	}
	for _, problem := range ref.conforms(route, rec.Code, rec.Header().Get("Content-Type"), rec.Body.Bytes()) {
		t.Errorf("%s answered %d: %s", ref.named(route), rec.Code, problem)
	}
}

// HandlerConforms is Conforms for a test that calls a handler without a
// route's path: the route is the one `handler` answers. Where one handler
// answers several routes, the answer must be what one of them says.
func HandlerConforms(t T, file string, handler echo.HandlerFunc, rec *httptest.ResponseRecorder) {
	t.Helper()
	ref, err := loadReference(file)
	if err != nil {
		t.Fatalf("%v", err)
	}
	name := HandlerName(handler)
	routes := ref.routesOf(name)
	if len(routes) == 0 {
		t.Errorf("%s answers no route in %s", name, file)
		return
	}
	var said []string
	for _, route := range routes {
		problems := ref.conforms(route, rec.Code, rec.Header().Get("Content-Type"), rec.Body.Bytes())
		if len(problems) == 0 {
			return
		}
		for _, p := range problems {
			said = append(said, fmt.Sprintf("%s answered %d: %s", route, rec.Code, p))
		}
	}
	for _, s := range said {
		t.Errorf("%s", s)
	}
}

// routesOf is every route a handler answers, as "GET /path".
func (ref *reference) routesOf(handler string) []string {
	var routes []string
	for path, item := range ref.Paths {
		for method, op := range item {
			if handler != "" && op.Handler == handler {
				routes = append(routes, strings.ToUpper(method)+" "+path)
			}
		}
	}
	sort.Strings(routes)
	return routes
}

type reference struct {
	Paths      map[string]map[string]operation `json:"paths"`
	Components struct {
		Schemas map[string]map[string]any `json:"schemas"`
	} `json:"components"`
}

type operation struct {
	Handler   string `json:"x-handler"`
	Responses map[string]struct {
		Content map[string]struct {
			Schema map[string]any `json:"schema"`
		} `json:"content"`
	} `json:"responses"`
}

var references sync.Map

func loadReference(file string) (*reference, error) {
	if ref, ok := references.Load(file); ok {
		return ref.(*reference), nil
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading the reference: %w", err)
	}
	ref := &reference{}
	if err := json.Unmarshal(raw, ref); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	references.Store(file, ref)
	return ref, nil
}

// conforms is what is wrong with an answer, or nothing.
func (ref *reference) conforms(route string, status int, contentType string, body []byte) []string {
	method, path, ok := strings.Cut(route, " ")
	if !ok {
		return []string{"the route is its method and its path: \"GET /hashtags/:alias\""}
	}
	op, _, ok := ref.operation(strings.ToLower(method), path)
	if !ok {
		return []string{"the reference has no such route"}
	}
	if status == http.StatusInternalServerError {
		return nil
	}

	answer, ok := op.Responses[strconv.Itoa(status)]
	if !ok {
		listed := make([]string, 0, len(op.Responses))
		for code := range op.Responses {
			listed = append(listed, code)
		}
		sort.Strings(listed)
		return []string{"the reference lists " + strings.Join(listed, ", ") + " and not this"}
	}

	empty := len(bytes.TrimSpace(body)) == 0
	if len(answer.Content) == 0 {
		if !empty {
			return []string{"the reference says it is sent with nothing in it, and it has a body"}
		}
		return nil
	}
	if empty {
		// Said in words alongside a body: "or, without a body, …".
		return nil
	}

	media, _, _ := mime.ParseMediaType(contentType)
	content, ok := answer.Content[media]
	if !ok {
		listed := make([]string, 0, len(answer.Content))
		for m := range answer.Content {
			listed = append(listed, m)
		}
		sort.Strings(listed)
		return []string{fmt.Sprintf("it is %q, and the reference says %s", media, strings.Join(listed, ", "))}
	}
	if !strings.HasSuffix(media, "json") {
		return nil
	}

	var value any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return []string{"the body is not JSON: " + err.Error()}
	}
	var problems []string
	ref.check(content.Schema, value, "the body", &problems, 0)
	return problems
}

// operation finds a route by its method and its path: a route's own
// ("/cases/:id"), or one a request was sent to ("/cases/1?status=open").
// A parameter is one segment, whatever it is called; where two routes
// match, the one with more fixed segments is it, as echo chooses.
func (ref *reference) operation(method, path string) (operation, string, bool) {
	path, _, _ = strings.Cut(path, "?")
	want := strings.Split(shape(path), "/")
	best, found, at := -1, operation{}, ""
	for p, item := range ref.Paths {
		op, ok := item[method]
		if !ok {
			continue
		}
		if fixed, ok := matches(strings.Split(shape(p), "/"), want); ok && fixed > best {
			best, found, at = fixed, op, p
		}
	}
	return found, at, best >= 0
}

// named is a route as the reference writes it: "GET /notes/{id}" for
// "GET /notes/7". As it was given when there is no such route.
func (ref *reference) named(route string) string {
	method, path, _ := strings.Cut(route, " ")
	if _, at, ok := ref.operation(strings.ToLower(method), path); ok {
		return strings.ToUpper(method) + " " + at
	}
	return route
}

// matches is whether a route's segments match a path's, and how many of
// them are fixed.
func matches(route, path []string) (int, bool) {
	if len(route) != len(path) {
		return 0, false
	}
	fixed := 0
	for i, seg := range route {
		switch {
		case seg == path[i]:
			if seg != "{}" {
				fixed++
			}
		case seg == "{}" && path[i] != "":
		default:
			return 0, false
		}
	}
	return fixed, true
}

// shape is a path with each parameter written `{}`, whether echo's `:id`,
// the reference's `{id}`, or the rest of the address.
func shape(path string) string {
	path = wildcard(normalizePath(path))
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") || seg == "*" || (strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")) {
			segments[i] = "{}"
		}
	}
	return strings.Join(segments, "/")
}

// check adds to `problems` what is wrong with `value` as a `schema`.
func (ref *reference) check(schema map[string]any, value any, where string, problems *[]string, depth int) {
	if depth > 64 {
		return
	}
	if name, ok := schema["$ref"].(string); ok {
		target, ok := ref.Components.Schemas[strings.TrimPrefix(name, "#/components/schemas/")]
		if !ok {
			*problems = append(*problems, where+": the reference names "+name+" and does not describe it")
			return
		}
		ref.check(target, value, where, problems, depth+1)
		return
	}
	// Go writes a nil slice, map or pointer as null, and the reference
	// marks as required only what is always there.
	if value == nil {
		return
	}
	if options, ok := schema["anyOf"].([]any); ok {
		for _, option := range options {
			var tried []string
			if o, ok := option.(map[string]any); ok {
				ref.check(o, value, where, &tried, depth+1)
			}
			if len(tried) == 0 {
				return
			}
		}
		*problems = append(*problems, where+": is none of what it may be")
		return
	}
	if enum, ok := schema["enum"].([]any); ok && !oneOf(enum, value) {
		*problems = append(*problems, fmt.Sprintf("%s: %v is not one of %v", where, value, enum))
	}

	switch schemaType(schema) {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			*problems = append(*problems, where+": is not an object")
			return
		}
		properties, _ := schema["properties"].(map[string]any)
		required, _ := schema["required"].([]any)
		for _, r := range required {
			if name, _ := r.(string); name != "" {
				if _, ok := object[name]; !ok {
					*problems = append(*problems, fmt.Sprintf("%s: `%s` is missing, and the reference says it is always there", where, name))
				}
			}
		}
		extra, _ := schema["additionalProperties"].(map[string]any)
		for _, name := range sortedNames(object) {
			field := where + "." + name
			if p, ok := properties[name].(map[string]any); ok {
				ref.check(p, object[name], field, problems, depth+1)
				continue
			}
			if extra != nil {
				ref.check(extra, object[name], field, problems, depth+1)
				continue
			}
			if properties != nil || schema["additionalProperties"] == false {
				*problems = append(*problems, fmt.Sprintf("%s: `%s` is not in the reference", where, name))
			}
		}
	case "array":
		array, ok := value.([]any)
		if !ok {
			*problems = append(*problems, where+": is not an array")
			return
		}
		items, _ := schema["items"].(map[string]any)
		for i, item := range array {
			ref.check(items, item, fmt.Sprintf("%s[%d]", where, i), problems, depth+1)
		}
	case "string":
		if _, ok := value.(string); !ok {
			*problems = append(*problems, where+": is not a string")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			*problems = append(*problems, where+": is not true or false")
		}
	case "integer":
		n, ok := value.(json.Number)
		if !ok {
			*problems = append(*problems, where+": is not a number")
			return
		}
		if f, err := n.Float64(); err != nil || f != math.Trunc(f) {
			*problems = append(*problems, where+": is not a whole number")
		}
	case "number":
		if _, ok := value.(json.Number); !ok {
			*problems = append(*problems, where+": is not a number")
		}
	}
}

// schemaType is a schema's type, the one that is not null where it may
// be either.
func schemaType(schema map[string]any) string {
	switch t := schema["type"].(type) {
	case string:
		return t
	case []any:
		for _, each := range t {
			if s, _ := each.(string); s != "" && s != "null" {
				return s
			}
		}
	}
	return ""
}

func oneOf(enum []any, value any) bool {
	for _, e := range enum {
		if fmt.Sprint(e) == fmt.Sprint(value) {
			return true
		}
	}
	return false
}

func sortedNames(m map[string]any) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Answers collects the answers a package's handler tests get, to be
// checked against the reference once they have all run: for tests whose
// helpers call handlers with no *testing.T at hand.
//
//	var answers = apidoc.NewAnswers("../../../../docs/api/like-service.json")
//
//	func TestMain(m *testing.M) { os.Exit(answers.Run(m)) }
//
// and in a helper, after the handler has answered:
//
//	answers.Handler(h.Likes.Add, rec)
type Answers struct {
	file string

	mu   sync.Mutex
	list []recorded
}

// recorded is an answer as it was given, kept before a test reads the body
// away.
type recorded struct {
	route       string
	handler     echo.HandlerFunc
	status      int
	contentType string
	body        []byte
}

// NewAnswers collects answers to check against the reference in `file`.
func NewAnswers(file string) *Answers {
	return &Answers{file: file}
}

// Route keeps an answer `route` gave: "POST /totp", or the path a request
// was sent to.
func (a *Answers) Route(route string, rec *httptest.ResponseRecorder) {
	a.keep(recorded{route: route}, rec)
}

// Handler keeps an answer `handler` gave.
func (a *Answers) Handler(handler echo.HandlerFunc, rec *httptest.ResponseRecorder) {
	a.keep(recorded{handler: handler}, rec)
}

func (a *Answers) keep(r recorded, rec *httptest.ResponseRecorder) {
	r.status = rec.Code
	r.contentType = rec.Header().Get("Content-Type")
	r.body = append([]byte(nil), rec.Body.Bytes()...)
	a.mu.Lock()
	a.list = append(a.list, r)
	a.mu.Unlock()
}

// Problems is what is wrong with the answers kept, each said once.
func (a *Answers) Problems() []string {
	ref, err := loadReference(a.file)
	if err != nil {
		return []string{err.Error()}
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	seen := map[string]bool{}
	var said []string
	say := func(s string) {
		if !seen[s] {
			seen[s] = true
			said = append(said, s)
		}
	}
	for _, r := range a.list {
		routes := []string{r.route}
		if r.handler != nil {
			routes = ref.routesOf(HandlerName(r.handler))
			if len(routes) == 0 {
				say(HandlerName(r.handler) + " answers no route in " + a.file)
				continue
			}
		}
		var problems []string
		for _, route := range routes {
			found := ref.conforms(route, r.status, r.contentType, r.body)
			if len(found) == 0 {
				problems = nil
				break
			}
			for _, p := range found {
				problems = append(problems, fmt.Sprintf("%s answered %d: %s", ref.named(route), r.status, p))
			}
		}
		for _, p := range problems {
			say(p)
		}
	}
	return said
}

// Run runs a package's tests, then checks the answers they got, and is
// what TestMain exits with.
func (a *Answers) Run(m interface{ Run() int }) int {
	code := m.Run()
	problems := a.Problems()
	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "--- FAIL: the answers the tests got are not what the reference says (docs/api.md, Describing a route):")
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "    "+p)
		}
		return 1
	}
	return code
}
