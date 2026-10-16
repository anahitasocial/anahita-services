package apidoc

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The reference as a page for people: one HTML file, made from the
// combined document and each service's, that opens in a browser with the
// network off. Nothing is fetched: its styles and its one small script
// are in it.
//
// Each service is a section, `#<service>` ("#hashtag-service"); each route
// is `#<operationId>` ("#getHashtagsByAlias"); each shape is
// `#schema-<name>`. The prose pages link to them.

// CheckPage makes the page from the documents in `dir` and compares it
// with `file`, as Check does a document.
func CheckPage(t T, dir, combined, file string) {
	t.Helper()
	page, err := Page(dir, combined)
	if err != nil {
		t.Fatalf("making the page: %v", err)
	}
	compare(t, file, page)
}

type pageDoc struct {
	Info struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"info"`
	Tags []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"tags"`
	Paths      map[string]map[string]pageOp `json:"paths"`
	Components struct {
		Schemas map[string]map[string]any `json:"schemas"`
	} `json:"components"`
}

type pageOp struct {
	OperationID string                `json:"operationId"`
	Summary     string                `json:"summary"`
	Description string                `json:"description"`
	Tags        []string              `json:"tags"`
	Viewer      string                `json:"x-viewer"`
	StepUp      bool                  `json:"x-step-up"`
	Internal    bool                  `json:"x-internal"`
	Handler     string                `json:"x-handler"`
	Security    []map[string][]string `json:"security"`
	Parameters  []struct {
		Name        string         `json:"name"`
		In          string         `json:"in"`
		Required    bool           `json:"required"`
		Description string         `json:"description"`
		Schema      map[string]any `json:"schema"`
	} `json:"parameters"`
	RequestBody *struct {
		Content map[string]struct {
			Schema map[string]any `json:"schema"`
		} `json:"content"`
	} `json:"requestBody"`
	Responses map[string]struct {
		Description string `json:"description"`
		Content     map[string]struct {
			Schema map[string]any `json:"schema"`
		} `json:"content"`
	} `json:"responses"`
}

// Page is the page of the combined document `combined` in `dir`, with each
// service's title and description from its own file beside it.
func Page(dir, combined string) ([]byte, error) {
	var all pageDoc
	if err := readJSON(filepath.Join(dir, combined), &all); err != nil {
		return nil, err
	}
	about := map[string]string{}
	for _, tag := range all.Tags {
		var doc pageDoc
		if err := readJSON(filepath.Join(dir, tag.Name+".json"), &doc); err != nil {
			return nil, err
		}
		about[tag.Name] = doc.Info.Description
	}

	type route struct {
		method, path string
		op           pageOp
	}
	byService := map[string][]route{}
	for path, item := range all.Paths {
		for method, op := range item {
			for _, tag := range op.Tags {
				byService[tag] = append(byService[tag], route{method, path, op})
			}
		}
	}
	order := map[string]int{"get": 0, "post": 1, "put": 2, "patch": 3, "delete": 4}
	for _, routes := range byService {
		sort.Slice(routes, func(i, j int) bool {
			if routes[i].path != routes[j].path {
				return routes[i].path < routes[j].path
			}
			return order[routes[i].method] < order[routes[j].method]
		})
	}

	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	w("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	w("<title>Anahita API</title>\n<style>%s</style>\n</head>\n<body>\n", pageStyle)
	w("<header>\n<h1>Anahita API</h1>\n")
	w("<p>Every route of every Anahita service. What is true of every route, how lists are paged, how a refusal says why, and how to sign in, is in <a href=\"../api.md\">The API</a>. Made from the code by <code>make api_docs</code>: the same as <a href=\"anahita.json\">anahita.json</a>.</p>\n")
	w("<input id=\"filter\" type=\"search\" placeholder=\"Filter routes: a path, a word, a field\" aria-label=\"Filter routes\">\n</header>\n")

	w("<nav aria-label=\"Services\">\n<ul>\n")
	for _, tag := range all.Tags {
		w("<li><a href=\"#%s\">%s</a> <span class=\"count\">%d</span></li>\n", attr(tag.Name), html.EscapeString(tag.Description), len(byService[tag.Name]))
	}
	w("<li><a href=\"#schemas\">Shapes</a> <span class=\"count\">%d</span></li>\n</ul>\n</nav>\n<main>\n", len(all.Components.Schemas))

	for _, tag := range all.Tags {
		w("<section id=\"%s\" class=\"service\">\n<h2>%s <code>%s</code></h2>\n<p>%s</p>\n",
			attr(tag.Name), html.EscapeString(tag.Description), html.EscapeString(tag.Name), prose(about[tag.Name]))
		for _, r := range byService[tag.Name] {
			writeRoute(&b, r.method, r.path, r.op)
		}
		w("</section>\n")
	}

	w("<section id=\"schemas\" class=\"service\">\n<h2>Shapes</h2>\n<p>What the routes take and send, by name: one of anahita-libs by its own (<code>NodeItem</code>), one of a service with its module (<code>hashtag.HashtagNodeItem</code>).</p>\n")
	names := make([]string, 0, len(all.Components.Schemas))
	for name := range all.Components.Schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		schema := all.Components.Schemas[name]
		w("<div class=\"route schema\" id=\"schema-%s\" data-search=\"%s\">\n<h3>%s</h3>\n", attr(name), attr(strings.ToLower(name+" "+searchText(schema))), html.EscapeString(name))
		if d, _ := schema["description"].(string); d != "" {
			w("<p>%s</p>\n", prose(withoutName(d, name)))
		}
		writeFields(&b, schema)
		w("</div>\n")
	}
	w("</section>\n</main>\n<script>%s</script>\n</body>\n</html>\n", pageScript)
	return []byte(b.String()), nil
}

func writeRoute(b *strings.Builder, method, path string, op pageOp) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	text := strings.ToLower(method + " " + path + " " + op.Summary + " " + op.Description)
	w("<details class=\"route\" id=\"%s\" data-search=\"%s\">\n", attr(op.OperationID), attr(text))
	w("<summary><span class=\"method %s\">%s</span> <code class=\"path\">%s</code> <span class=\"summary\">%s</span></summary>\n",
		method, strings.ToUpper(method), html.EscapeString(path), html.EscapeString(op.Summary))

	description := op.Description
	// Who may call it is said below, from the fields; the sentence the
	// document adds to the description is not said twice.
	if i := strings.LastIndex(description, "Who may call it:"); i >= 0 {
		description = strings.TrimSpace(description[:i])
	}
	if description != "" {
		w("<div class=\"description\">%s</div>\n", paragraphs(description))
	}

	w("<p class=\"who\"><strong>Who:</strong> %s", html.EscapeString(viewerWords(op.Viewer)))
	for _, s := range op.Security {
		for _, scopes := range s {
			for _, scope := range scopes {
				w(", with <code>%s</code>", html.EscapeString(scope))
			}
		}
	}
	if op.StepUp {
		w(", who has just proved who they are again")
	}
	if op.Internal {
		w(". <strong>Inside the installation only.</strong>")
	}
	w("</p>\n")

	if len(op.Parameters) > 0 {
		w("<h4>Takes</h4>\n<table>\n<thead><tr><th>Name</th><th>In</th><th>Type</th><th>What it is</th></tr></thead>\n<tbody>\n")
		for _, p := range op.Parameters {
			required := ""
			if p.Required {
				required = " <span class=\"required\">required</span>"
			}
			w("<tr><td><code>%s</code>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
				html.EscapeString(p.Name), required, p.In, typeOf(p.Schema), prose(p.Description+constraints(p.Schema)))
		}
		w("</tbody>\n</table>\n")
	}
	if op.RequestBody != nil {
		// A form taken either way is one table.
		var shown []string
		same := map[string][]string{}
		for _, media := range sortedMedia(op.RequestBody.Content) {
			key, _ := json.Marshal(op.RequestBody.Content[media].Schema)
			if _, ok := same[string(key)]; !ok {
				shown = append(shown, string(key))
			}
			same[string(key)] = append(same[string(key)], media)
		}
		for _, key := range shown {
			media := same[key]
			names := make([]string, len(media))
			for i, m := range media {
				names[i] = "<code>" + html.EscapeString(m) + "</code>"
			}
			w("<h4>Takes, as %s</h4>\n", strings.Join(names, " or "))
			writeFields(b, op.RequestBody.Content[media[0]].Schema)
		}
	}

	w("<h4>Answers</h4>\n<table class=\"answers\">\n<tbody>\n")
	codes := make([]string, 0, len(op.Responses))
	for code := range op.Responses {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		r := op.Responses[code]
		body := ""
		for _, media := range sortedMedia(r.Content) {
			body += fmt.Sprintf("<div class=\"body\">%s <span class=\"media\">%s</span></div>", typeOf(r.Content[media].Schema), html.EscapeString(media))
		}
		w("<tr><td class=\"status s%c\">%s</td><td>%s%s</td></tr>\n", code[0], code, prose(r.Description), body)
	}
	w("</tbody>\n</table>\n")
	if op.Handler != "" {
		w("<p class=\"handler\">Answered by <code>%s</code></p>\n", html.EscapeString(op.Handler))
	}
	w("</details>\n")
}

// writeFields is a table of an object's fields, or what the schema is
// when it is not an object of its own.
func writeFields(b *strings.Builder, schema map[string]any) {
	properties, _ := schema["properties"].(map[string]any)
	if len(properties) == 0 {
		fmt.Fprintf(b, "<p class=\"body\">%s</p>\n", typeOf(schema))
		return
	}
	required := map[string]bool{}
	if list, ok := schema["required"].([]any); ok {
		for _, r := range list {
			if name, ok := r.(string); ok {
				required[name] = true
			}
		}
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Fprintf(b, "<table>\n<thead><tr><th>Field</th><th>Type</th><th>What it is</th></tr></thead>\n<tbody>\n")
	for _, name := range names {
		field, _ := properties[name].(map[string]any)
		mark := ""
		if required[name] {
			mark = " <span class=\"required\">always</span>"
		}
		description, _ := field["description"].(string)
		fmt.Fprintf(b, "<tr><td><code>%s</code>%s</td><td>%s</td><td>%s</td></tr>\n",
			html.EscapeString(name), mark, typeOf(field), prose(description+constraints(field)))
	}
	fmt.Fprintf(b, "</tbody>\n</table>\n")
}

// typeOf is a schema's type in words, with a link to a named shape.
func typeOf(schema map[string]any) string {
	if schema == nil {
		return "anything"
	}
	if ref, ok := schema["$ref"].(string); ok {
		name := strings.TrimPrefix(ref, "#/components/schemas/")
		return fmt.Sprintf("<a href=\"#schema-%s\">%s</a>", attr(name), html.EscapeString(name))
	}
	if options, ok := schema["anyOf"].([]any); ok {
		words := make([]string, 0, len(options))
		for _, o := range options {
			if m, ok := o.(map[string]any); ok {
				words = append(words, typeOf(m))
			}
		}
		return "one of " + strings.Join(words, ", ")
	}
	t := schemaType(schema)
	if list, ok := schema["type"].([]any); ok && len(list) > 1 {
		plain := map[string]any{}
		for k, v := range schema {
			plain[k] = v
		}
		plain["type"] = t
		return typeOf(plain) + " or null"
	}
	switch t {
	case "array":
		items, _ := schema["items"].(map[string]any)
		return "list of " + typeOf(items)
	case "object":
		if extra, ok := schema["additionalProperties"].(map[string]any); ok {
			return "map of " + typeOf(extra)
		}
		if properties, ok := schema["properties"].(map[string]any); ok && len(properties) > 0 {
			names := make([]string, 0, len(properties))
			for name := range properties {
				names = append(names, name)
			}
			sort.Strings(names)
			parts := make([]string, 0, len(names))
			for _, name := range names {
				p, _ := properties[name].(map[string]any)
				parts = append(parts, "<code>"+html.EscapeString(name)+"</code>: "+typeOf(p))
			}
			return "object { " + strings.Join(parts, ", ") + " }"
		}
		return "object"
	case "":
		return "anything"
	}
	if format, ok := schema["format"].(string); ok {
		switch format {
		case "date-time":
			return "time"
		case "binary":
			return "file"
		case "int64", "int32":
			return "integer"
		}
		return t + " (" + html.EscapeString(format) + ")"
	}
	return t
}

// constraints is what a value may be, in words, after its description.
func constraints(schema map[string]any) string {
	var said []string
	if enum, ok := schema["enum"].([]any); ok {
		values := make([]string, 0, len(enum))
		for _, v := range enum {
			values = append(values, fmt.Sprintf("`%v`", v))
		}
		said = append(said, "One of "+strings.Join(values, ", ")+".")
	}
	if d, ok := schema["default"]; ok {
		said = append(said, fmt.Sprintf("`%v` unless given.", d))
	}
	for key, words := range map[string]string{"minimum": "At least", "maximum": "At most", "minLength": "At least", "maxLength": "At most", "minItems": "At least", "maxItems": "At most"} {
		if v, ok := schema[key]; ok {
			unit := ""
			switch key {
			case "minLength", "maxLength":
				unit = " characters"
			case "minItems", "maxItems":
				unit = " of them"
			}
			said = append(said, fmt.Sprintf("%s %v%s.", words, v, unit))
		}
	}
	sort.Strings(said)
	if len(said) == 0 {
		return ""
	}
	return " " + strings.Join(said, " ")
}

func viewerWords(viewer string) string {
	switch viewer {
	case "anyone":
		return "anyone, signed in or not"
	case "signed_in":
		return "somebody signed in"
	case "admin":
		return "an administrator"
	case "super_admin":
		return "a super administrator"
	}
	return viewer
}

func sortedMedia[T any](content map[string]T) []string {
	media := make([]string, 0, len(content))
	for m := range content {
		media = append(media, m)
	}
	sort.Strings(media)
	return media
}

var (
	codeSpan = regexp.MustCompile("`([^`]+)`")
	strong   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdLink   = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	// A prose page named in passing: "(oauth.md)".
	mdPage = regexp.MustCompile(`(^|[\s(])([a-z0-9][a-z0-9-]*\.md)\b`)
)

// prose is a description as HTML: escaped, with its `code`, **strong** and
// [links](…) kept.
func prose(text string) string {
	s := html.EscapeString(text)
	s = codeSpan.ReplaceAllString(s, "<code>$1</code>")
	s = strong.ReplaceAllString(s, "<strong>$1</strong>")
	if !strings.Contains(s, "](") {
		s = mdPage.ReplaceAllString(s, `$1<a href="../$2">$2</a>`)
	}
	s = mdLink.ReplaceAllStringFunc(s, func(m string) string {
		parts := mdLink.FindStringSubmatch(m)
		href := parts[2]
		// The prose pages are beside docs/api/, one directory up.
		if !strings.Contains(href, "://") && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "../") {
			href = "../" + href
		}
		return "<a href=\"" + href + "\">" + parts[1] + "</a>"
	})
	return s
}

// paragraphs is prose with a paragraph for each blank-line break.
func paragraphs(text string) string {
	var b strings.Builder
	for _, p := range strings.Split(text, "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			b.WriteString("<p>" + prose(p) + "</p>")
		}
	}
	return b.String()
}

// withoutName is a shape's description without the Go name it begins
// with: "TotpStatus is whether …" is "Whether …".
func withoutName(description, name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	rest, ok := strings.CutPrefix(description, name+" is ")
	if !ok || rest == "" {
		return description
	}
	return strings.ToUpper(rest[:1]) + rest[1:]
}

// searchText is what the filter finds a shape by: its fields' names.
func searchText(schema map[string]any) string {
	properties, _ := schema["properties"].(map[string]any)
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}

func attr(s string) string {
	return html.EscapeString(s)
}

func readJSON(file string, into any) error {
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	return nil
}

const pageStyle = `
:root { color-scheme: light dark; --bg: #fff; --fg: #1d1d1f; --muted: #6b6b70; --line: #e3e3e8; --panel: #f6f6f8; --link: #0b57d0;
  --get: #1a7f37; --post: #0b57d0; --put: #8250df; --patch: #9a6700; --delete: #cf222e; }
@media (prefers-color-scheme: dark) { :root { --bg: #151517; --fg: #ececf0; --muted: #a0a0a8; --line: #2e2e33; --panel: #1e1e22; --link: #8ab4f8;
  --get: #56d364; --post: #79b8ff; --put: #d2a8ff; --patch: #e3b341; --delete: #ff7b72; } }
* { box-sizing: border-box; }
body { margin: 0; background: var(--bg); color: var(--fg); font: 15px/1.5 system-ui, -apple-system, "Segoe UI", sans-serif; }
a { color: var(--link); }
code { font: 13px/1.4 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
header, nav, main { max-width: 960px; margin: 0 auto; padding: 0 16px; }
header { padding-top: 24px; }
h1 { margin: 0 0 8px; }
#filter { width: 100%; padding: 8px 12px; margin: 8px 0 16px; font: inherit; color: inherit; background: var(--panel); border: 1px solid var(--line); border-radius: 8px; }
nav ul { list-style: none; padding: 0; margin: 0 0 24px; display: flex; flex-wrap: wrap; gap: 8px; }
nav li a { display: inline-block; padding: 4px 8px; border: 1px solid var(--line); border-radius: 8px; text-decoration: none; }
.count { color: var(--muted); font-size: 12px; }
.service { margin-bottom: 40px; }
.service h2 code { font-size: 14px; color: var(--muted); font-weight: normal; }
.route { border: 1px solid var(--line); border-radius: 8px; margin: 8px 0; padding: 0 16px; background: var(--panel); }
.route[open] { padding-bottom: 8px; }
summary { cursor: pointer; padding: 8px 0; display: flex; flex-wrap: wrap; gap: 8px; align-items: baseline; }
.method { font: bold 12px ui-monospace, monospace; min-width: 56px; }
.method.get { color: var(--get); } .method.post { color: var(--post); } .method.put { color: var(--put); }
.method.patch { color: var(--patch); } .method.delete { color: var(--delete); }
.path { word-break: break-all; }
.summary { color: var(--muted); }
table { border-collapse: collapse; width: 100%; margin: 8px 0; display: block; overflow-x: auto; }
th, td { text-align: left; vertical-align: top; padding: 4px 8px; border-top: 1px solid var(--line); }
th { color: var(--muted); font-weight: normal; font-size: 13px; }
td:first-child { white-space: nowrap; }
.required { color: var(--muted); font-size: 12px; }
.status { font: bold 13px ui-monospace, monospace; }
.status.s2 { color: var(--get); } .status.s3 { color: var(--post); } .status.s4 { color: var(--patch); } .status.s5 { color: var(--delete); }
.media, .handler { color: var(--muted); font-size: 12px; }
h3 { margin: 8px 0; font-size: 15px; }
h4 { margin: 16px 0 0; font-size: 13px; color: var(--muted); font-weight: normal; text-transform: uppercase; letter-spacing: .04em; }
.schema { padding: 8px 16px; }
.hidden { display: none; }
`

const pageScript = `
(function () {
  var input = document.getElementById('filter');
  var items = document.querySelectorAll('[data-search]');
  input.addEventListener('input', function () {
    var words = input.value.toLowerCase().split(/\s+/).filter(Boolean);
    items.forEach(function (item) {
      var text = item.getAttribute('data-search');
      var shown = words.every(function (w) { return text.indexOf(w) >= 0; });
      item.classList.toggle('hidden', !shown);
    });
  });
  // A link to a route opens it.
  function open() {
    var target = location.hash && document.getElementById(decodeURIComponent(location.hash.slice(1)));
    if (target && target.tagName === 'DETAILS') { target.open = true; target.scrollIntoView(); }
  }
  window.addEventListener('hashchange', open);
  open();
})();
`
