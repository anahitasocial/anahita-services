// Package about is what an installation says about itself (roadmap row 102):
// its rules, which everybody agrees to by ticking each one; its promises,
// shown on its About page; and the rules offered to a new group's
// administrators to start from.
//
// They are the operator's, in a JSON file like the other defaults
// (SITE_DEFAULTS_PATH, /app/defaults/site.json in the image; mounted by
// docker-compose from auth-defaults/site.json, a ConfigMap on Kubernetes). It
// is read again when it changes. What it leaves out is as it comes
// (default.json, built in), and a file that cannot be understood is not
// taken: the built-in one is used, and the log says so.
//
// Every text carries its own translations, by the web app's language names
// ("en-CA", "fr-FR", "de-DE", "es-419", "pt-BR"). A language left out falls
// back to another of its own, then to English: an operator may write in one
// language.
package about

import (
	_ "embed"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultPath is where the image keeps the file.
const DefaultPath = "/app/defaults/site.json"

// EnvPath names the setting that says where it is.
const EnvPath = "SITE_DEFAULTS_PATH"

// MaxItems is the most rules or promises a list may have: every line of a
// checklist is to be read (the user, 2026-10-19).
const MaxItems = 10

//go:embed default.json
var builtIn []byte

// Text is some words in each language they are written in.
type Text map[string]string

// In is the words in `language`, or the nearest there is: the same language
// as spoken elsewhere ("fr-CA" for "fr-FR"), then English, then any.
func (t Text) In(language string) string {
	if words, ok := t[language]; ok && words != "" {
		return words
	}
	base := strings.SplitN(language, "-", 2)[0]
	for key, words := range t {
		if strings.SplitN(key, "-", 2)[0] == base && words != "" {
			return words
		}
	}
	for key, words := range t {
		if strings.HasPrefix(key, "en") && words != "" {
			return words
		}
	}
	for _, words := range t {
		if words != "" {
			return words
		}
	}
	return ""
}

// Item is one rule, promise or suggestion: a short title, and a line more if
// it needs one.
type Item struct {
	Title Text `json:"title"`
	Body  Text `json:"body,omitempty"`
}

// Rules is the site's rules, with the version agreed to. Raising the
// version asks everybody to agree again; a correction that leaves it as it
// is asks nobody.
type Rules struct {
	Version string `json:"version"`
	Items   []Item `json:"items"`
}

// Site is all of what the file says.
type Site struct {
	Rules           Rules  `json:"rules"`
	Promises        []Item `json:"promises"`
	RuleSuggestions []Item `json:"rule_suggestions"`
}

// Default is what an installation says until its operator says otherwise.
func Default() Site {
	var site Site
	if err := json.Unmarshal(builtIn, &site); err != nil {
		panic("about: the built-in default.json is not understood: " + err.Error())
	}
	return site
}

// Parse reads a file's words over the defaults: what it leaves out is as it
// comes. False when it is not understood, or says what cannot be.
func Parse(raw []byte) (Site, bool) {
	var given struct {
		Rules           *Rules  `json:"rules"`
		Promises        *[]Item `json:"promises"`
		RuleSuggestions *[]Item `json:"rule_suggestions"`
	}
	if err := json.Unmarshal(raw, &given); err != nil {
		return Default(), false
	}
	site := Default()
	if given.Rules != nil {
		site.Rules = *given.Rules
	}
	if given.Promises != nil {
		site.Promises = *given.Promises
	}
	if given.RuleSuggestions != nil {
		site.RuleSuggestions = *given.RuleSuggestions
	}
	if !site.valid() {
		return Default(), false
	}
	return site, true
}

func (s Site) valid() bool {
	if len(s.Rules.Items) > 0 && !validVersion(s.Rules.Version) {
		return false
	}
	for _, list := range [][]Item{s.Rules.Items, s.Promises} {
		if len(list) > MaxItems {
			return false
		}
		for _, item := range list {
			if item.Title.In("en-CA") == "" {
				return false
			}
		}
	}
	return true
}

// validVersion is digits and dots, up to three parts, as the Terms'
// versions are: "1", "1.1", "2.0.1".
func validVersion(version string) bool {
	parts := strings.Split(version, ".")
	if version == "" || len(parts) > 3 {
		return false
	}
	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil || part == "" || strings.HasPrefix(part, "-") {
			return false
		}
	}
	return true
}

// Filled is the site with what its words leave to the configuration filled
// in: {{days}}, the days a deleted account can be brought back.
func (s Site) Filled(days int) Site {
	fill := func(items []Item) []Item {
		out := make([]Item, 0, len(items))
		for _, item := range items {
			body := Text{}
			for language, words := range item.Body {
				body[language] = strings.ReplaceAll(words, "{{days}}", strconv.Itoa(days))
			}
			out = append(out, Item{Title: item.Title, Body: body})
		}
		return out
	}
	s.Promises = fill(s.Promises)
	s.Rules.Items = fill(s.Rules.Items)
	return s
}

// Source is the file, read again when it changes.
type Source struct {
	path string

	mu       sync.Mutex
	modified time.Time
	site     Site
}

// NewSource is the site read from `path`.
func NewSource(path string) *Source {
	return &Source{path: path, site: Default()}
}

// FromEnv is the site read from SITE_DEFAULTS_PATH, or the image's file.
func FromEnv() *Source {
	path := os.Getenv(EnvPath)
	if path == "" {
		path = DefaultPath
	}
	return NewSource(path)
}

// Site is what the file says now.
func (s *Source) Site() Site {
	if s == nil {
		return Default()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	if err != nil {
		// No file: as it comes.
		s.site, s.modified = Default(), time.Time{}
		return s.site
	}
	if info.ModTime().Equal(s.modified) {
		return s.site
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return s.site
	}
	site, ok := Parse(raw)
	if !ok {
		log.Printf("about: %s is not understood; what the site says is as it comes", s.path)
	}
	s.site, s.modified = site, info.ModTime()
	return s.site
}
