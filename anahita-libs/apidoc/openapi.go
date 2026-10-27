package apidoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// The OpenAPI 3.1 document of a service.

// Document is a service's routes as an OpenAPI 3.1 document, written as it
// is committed. `dir` is a directory of the service's module, from which its
// source is found.
func (s *Service) Document(dir string) ([]byte, error) {
	src := newSources(dir)
	schemas := newSchemas(src)

	paths := map[string]any{}
	operationIDs := map[string]bool{}
	// One name for each place in a path where a value goes, whatever each
	// route calls it there: OpenAPI does not allow /hashtags/{alias} and
	// /hashtags/{id} side by side, as echo does.
	names := map[string][]string{}

	for _, route := range s.routes {
		if err := route.complete(); err != nil {
			return nil, err
		}
		key, rename := pathKey(route.Path, names)
		op, err := route.operation(schemas, rename)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", route.Method, route.Path, err)
		}
		// A handler registered twice, with and without a slash at the end,
		// is two routes of one name: the second is numbered.
		id := operationID(route.Method, route.Path)
		for n := 2; operationIDs[id]; n++ {
			id = fmt.Sprintf("%s%d", operationID(route.Method, route.Path), n)
		}
		operationIDs[id] = true
		op["operationId"] = id
		op["tags"] = []any{s.Name}

		item, _ := paths[key].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[key] = item
		}
		method := strings.ToLower(route.Method)
		if _, taken := item[method]; taken {
			return nil, fmt.Errorf("%s %s is described twice", route.Method, route.Path)
		}
		item[method] = op
	}

	doc := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       s.Title,
			"description": s.Description,
			"version":     "dev",
			"license":     license,
		},
		"servers": servers,
		"tags":    []any{map[string]any{"name": s.Name, "description": s.Title}},
		"paths":   paths,
		"components": map[string]any{
			"schemas":         schemas.components,
			"securitySchemes": securitySchemes,
		},
	}
	return encode(doc)
}

// The services' licence, since the API is theirs (docs/licensing.md).
var license = map[string]any{"name": "GNU Affero General Public License v3.0 only", "identifier": "AGPL-3.0-only"}

// Every installation has an API of its own, at an address of its own.
var servers = []any{map[string]any{
	"url":         "https://{host}",
	"description": "An installation's API",
	"variables": map[string]any{
		"host": map[string]any{"default": "api.example.org", "description": "Where the installation's API is"},
	},
}}

var securitySchemes = map[string]any{
	"bearer": map[string]any{
		"type":         "http",
		"scheme":       "bearer",
		"bearerFormat": "JWT",
		"description":  "An access token from auth-service (docs/oauth.md).",
	},
}

// encode writes a document as it is committed: indented, keys in order,
// and < > & left as they are.
func encode(doc any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// complete says what a route's description lacks.
func (r *Route) complete() error {
	var lacks []string
	if r.summary == "" {
		lacks = append(lacks, "a summary")
	}
	if r.access == "" {
		lacks = append(lacks, "who may call it (Viewer)")
	}
	if len(r.sends) == 0 && r.notBuilt == "" {
		lacks = append(lacks, "what it sends (Sends or SendsNothing)")
	}
	if len(lacks) > 0 {
		return fmt.Errorf("%s %s has no %s", r.Method, r.Path, strings.Join(lacks, ", no "))
	}
	return nil
}

// pathKey is a route's path as OpenAPI writes it ("/hashtags/{alias}"), and
// what each of its parameters is called there.
func pathKey(path string, names map[string][]string) (string, map[string]string) {
	segments := strings.Split(wildcard(path), "/")
	var template []string
	var params []string
	for _, seg := range segments {
		switch {
		case strings.HasPrefix(seg, ":"):
			template = append(template, "{}")
			params = append(params, seg[1:])
		case seg == "*":
			// The rest of the address: a file under a directory.
			template = append(template, "{path}")
		default:
			template = append(template, seg)
		}
	}
	shape := strings.Join(template, "/")
	known, ok := names[shape]
	if !ok {
		known = params
		names[shape] = params
	}

	rename := map[string]string{}
	i := 0
	for n, seg := range segments {
		switch {
		case strings.HasPrefix(seg, ":"):
			rename[seg[1:]] = known[i]
			segments[n] = "{" + known[i] + "}"
			i++
		case seg == "*":
			segments[n] = "{path}"
		}
	}
	return strings.Join(segments, "/"), rename
}

// wildcard is a path whose end is the rest of the address, "/static*" as
// echo registers a directory of files, written as a segment of its own:
// "/static/*".
func wildcard(path string) string {
	if strings.HasSuffix(path, "*") && !strings.HasSuffix(path, "/*") {
		return strings.TrimSuffix(path, "*") + "/*"
	}
	return path
}

// operationID is a name for a route that a client generator can make a
// method of: GET /hashtags/:alias is getHashtagsByAlias.
func operationID(method, path string) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(method))
	for _, seg := range strings.Split(wildcard(path), "/") {
		by := strings.HasPrefix(seg, ":")
		if seg == "*" {
			by, seg = true, "path"
		}
		seg = strings.TrimLeft(seg, ":_")
		if seg == "" {
			continue
		}
		if by {
			b.WriteString("By")
		}
		for _, word := range strings.FieldsFunc(seg, func(r rune) bool { return r == '_' || r == '-' || r == '.' }) {
			b.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
	}
	return b.String()
}

var accessWords = map[Access]string{
	Anyone:     "anyone, signed in or not",
	SignedIn:   "somebody signed in",
	Admin:      "a site administrator",
	SuperAdmin: "a super administrator",
}

func (r *Route) operation(schemas *schemas, rename map[string]string) (map[string]any, error) {
	op := map[string]any{"summary": r.summary}

	who := "Who may call it: " + accessWords[r.access]
	if r.scope != "" {
		who += ", with a token that carries `" + r.scope + "`"
	}
	if r.stepUp {
		who += ", who has just proved who they are again"
	}
	description := who + "."
	if r.description != "" {
		description = r.description + "\n\n" + description
	}
	if r.notBuilt != "" {
		description = "**Not built:** " + r.notBuilt + "\n\n" + description
	}
	if r.internal != "" {
		description = "**Inside the installation only:** " + r.internal + " The gateway sends nobody from outside here.\n\n" + description
		op["x-internal"] = true
	}
	op["description"] = description
	op["x-viewer"] = string(r.access)

	switch {
	case r.access == Anyone:
		op["security"] = []any{map[string]any{}, map[string]any{"bearer": []any{}}}
	case r.scope != "":
		op["security"] = []any{map[string]any{"bearer": []any{r.scope}}}
	default:
		op["security"] = []any{map[string]any{"bearer": []any{}}}
	}
	if r.stepUp {
		op["x-step-up"] = true
	}
	if r.handler != "" {
		op["x-handler"] = r.handler
	}

	canBeMalformed := false
	if r.takes.IsValid() {
		params, body, checked, err := r.request(schemas, rename)
		if err != nil {
			return nil, err
		}
		if len(params) > 0 {
			op["parameters"] = params
		}
		if body != nil {
			op["requestBody"] = body
		}
		canBeMalformed = checked && !r.unbound
	}
	mediaType := "application/json"
	if r.media != "" {
		mediaType = r.media
	}
	if r.body != nil {
		schema, err := schemas.of(r.body)
		if err != nil {
			return nil, err
		}
		op["requestBody"] = map[string]any{"required": true, "content": map[string]any{mediaType: map[string]any{"schema": schema}}}
		canBeMalformed = true
	}
	if strings.HasSuffix(r.Path, "*") {
		params, _ := op["parameters"].([]any)
		op["parameters"] = append(params, map[string]any{
			"name": "path", "in": "path", "required": true,
			"description": "The rest of the address: the file's path under the directory.",
			"schema":      map[string]any{"type": "string"},
		})
	}
	if err := r.checkPathParams(op, rename); err != nil {
		return nil, err
	}

	responses := map[string]any{}
	if r.notBuilt != "" {
		responses["501"] = map[string]any{"description": r.notBuilt}
		op["responses"] = responses
		return op, nil
	}
	for _, a := range r.sends {
		code := strconv.Itoa(a.status)
		if _, taken := responses[code]; taken {
			return nil, fmt.Errorf("sends %d twice", a.status)
		}
		resp := map[string]any{"description": a.reason}
		if a.reason == "" {
			resp["description"] = http.StatusText(a.status)
		}
		if a.body != nil {
			schema, err := schemas.of(a.body)
			if err != nil {
				return nil, err
			}
			resp["content"] = map[string]any{mediaType: map[string]any{"schema": schema}}
		}
		responses[code] = resp
	}

	// What is refused, and why. Several reasons for one status are one
	// answer, as they are on the wire.
	reasons := map[int][]string{}
	for _, a := range r.refuses {
		reasons[a.status] = append(reasons[a.status], a.reason)
	}
	// What every route of its kind refuses, unless it says otherwise: in
	// a refusal of its own, or in an answer with a body on that status.
	said := func(status int) bool {
		_, refused := reasons[status]
		_, sent := responses[strconv.Itoa(status)]
		return refused || sent
	}
	if !said(http.StatusBadRequest) && canBeMalformed {
		reasons[http.StatusBadRequest] = []string{"the request is not well formed, or a value in it is not one it may be"}
	}
	if !said(http.StatusUnauthorized) && r.access != Anyone {
		reasons[http.StatusUnauthorized] = []string{"no token, or one that has expired"}
	}
	if !said(http.StatusForbidden) && (r.access == Admin || r.access == SuperAdmin || r.scope != "") {
		reasons[http.StatusForbidden] = []string{"the viewer may not do this"}
	}
	statuses := make([]int, 0, len(reasons))
	for status := range reasons {
		statuses = append(statuses, status)
	}
	sort.Ints(statuses)
	for _, status := range statuses {
		code := strconv.Itoa(status)
		// Sent with a body for some reasons and refused without one for
		// others: one answer, with every reason.
		if sent, taken := responses[code].(map[string]any); taken {
			sent["description"] = sent["description"].(string) + "; or, without a body: " + strings.Join(reasons[status], "; ")
			continue
		}
		responses[code] = map[string]any{"description": strings.Join(reasons[status], "; ")}
	}
	op["responses"] = responses
	return op, nil
}

// request is what a route takes, read from the struct its handler binds:
// its path, query and header values, and its body. `checked` says whether
// binding or validating it can refuse a request: a value with a rule, one in
// the query or the body, or one in the path that is not plain text.
func (r *Route) request(schemas *schemas, rename map[string]string) (params []any, body map[string]any, checked bool, err error) {
	v := r.takes
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v = reflect.New(v.Type().Elem()).Elem()
			break
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, nil, false, fmt.Errorf("takes a %s, not a struct", v.Type())
	}

	jsonBody := map[string]any{}
	var jsonRequired []string
	formBody := map[string]any{}
	var formRequired []string
	hasBody := r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch

	err = r.eachField(schemas, v, func(f reflect.StructField, value reflect.Value, comment string) error {
		schema, err := schemas.of(f.Type)
		if err != nil {
			return err
		}
		if comment != "" {
			schema = withDescription(schema, comment)
		}
		if f.Tag.Get("validate") != "" {
			checked = true
		}
		required, other := validated(schema, f.Type, f.Tag.Get("validate"))
		if len(other) > 0 {
			schema["x-validate"] = strings.Join(other, ",")
		}
		if isBasic(value) && !value.IsZero() {
			schema["default"] = value.Interface()
		}

		param := func(name, in string) {
			p := map[string]any{"name": name, "in": in, "schema": schema}
			if d, ok := schema["description"]; ok {
				p["description"] = d
				delete(schema, "description")
			}
			if in == "path" || required {
				p["required"] = true
			}
			params = append(params, p)
		}

		switch {
		case f.Tag.Get("param") != "":
			name := f.Tag.Get("param")
			// A field for a value that is not in this route's path: the
			// struct is shared with routes that have it.
			if as, ok := rename[name]; ok {
				if derefKind(f.Type) != reflect.String {
					checked = true
				}
				// Called by another route's name for the same place: said,
				// so that "alias" holding an id does not mislead.
				if as != name {
					note := "Here, the `" + name + "`."
					if d, ok := schema["description"].(string); ok {
						note += " " + d
					}
					schema["description"] = note
				}
				param(as, "path")
			}
		case f.Tag.Get("query") != "":
			checked = true
			param(f.Tag.Get("query"), "query")
		case f.Tag.Get("header") != "":
			param(f.Tag.Get("header"), "header")
		case !hasBody:
			// Nothing else is read from a request without a body.
		default:
			// A field may be sent as JSON, in a form, or either way: by
			// its tags. One tagged "-" is not read that way; one with no
			// tag is read from JSON by its Go name.
			formTag, hasForm := f.Tag.Lookup("form")
			jsonTag, _ := jsonName(f)
			if hasForm && formTag != "-" {
				formBody[formTag] = schema
				if required {
					formRequired = append(formRequired, formTag)
				}
			}
			if jsonTag != "-" && (jsonTag != "" || !hasForm) {
				if jsonTag == "" {
					jsonTag = f.Name
				}
				jsonBody[jsonTag] = schema
				if required {
					jsonRequired = append(jsonRequired, jsonTag)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, false, err
	}

	content := map[string]any{}
	if len(jsonBody) > 0 {
		content["application/json"] = map[string]any{"schema": objectOf(jsonBody, jsonRequired)}
	}
	if len(formBody) > 0 {
		form := map[string]any{"schema": objectOf(formBody, formRequired)}
		content["multipart/form-data"] = form
		// What a page's form posts, unless it sends a file: echo reads
		// either.
		if !sendsFile(formBody) {
			content["application/x-www-form-urlencoded"] = form
		}
	}
	if len(content) > 0 {
		body = map[string]any{"content": content, "required": true}
		checked = true
	}
	return params, body, checked, nil
}

// sendsFile is whether a form has a file in it.
func sendsFile(props map[string]any) bool {
	for _, p := range props {
		if schema, ok := p.(map[string]any); ok && schema["format"] == "binary" {
			return true
		}
	}
	return false
}

func objectOf(props map[string]any, required []string) map[string]any {
	o := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		o["required"] = required
	}
	return o
}

// eachField is every field of a request struct, those of embedded structs
// as its own, with the comment on it.
func (r *Route) eachField(schemas *schemas, v reflect.Value, do func(reflect.StructField, reflect.Value, string) error) error {
	t := v.Type()
	comments, err := schemas.src.comments(t.PkgPath(), t.Name())
	if err != nil {
		return err
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		value := v.Field(i)
		if f.Anonymous {
			for value.Kind() == reflect.Pointer {
				if value.IsNil() {
					value = reflect.New(value.Type().Elem()).Elem()
					break
				}
				value = value.Elem()
			}
			if value.Kind() == reflect.Struct {
				if err := r.eachField(schemas, value, do); err != nil {
					return err
				}
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		comment := ""
		if comments != nil {
			comment = comments.fields[f.Name]
		}
		if err := do(f, value, comment); err != nil {
			return fmt.Errorf("%s.%s: %w", t, f.Name, err)
		}
	}
	return nil
}

// checkPathParams says when a value in the path is not described.
func (r *Route) checkPathParams(op map[string]any, rename map[string]string) error {
	described := map[string]bool{}
	params, _ := op["parameters"].([]any)
	for _, p := range params {
		p := p.(map[string]any)
		if p["in"] == "path" {
			described[p["name"].(string)] = true
		}
	}
	var missing []string
	for _, as := range rename {
		if !described[as] {
			missing = append(missing, as)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("the path's %s not described: Takes a struct with a `param` field for each", strings.Join(missing, ", "))
	}
	return nil
}

func derefKind(t reflect.Type) reflect.Kind {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind()
}

func isBasic(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}
