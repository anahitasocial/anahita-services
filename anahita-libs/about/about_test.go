package about

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The operator's copy and the built-in one say the same, so an installation
// that mounts nothing says what one that mounts the file says.
func TestTheDefaultsFileIsTheBuiltInOne(t *testing.T) {
	theirs, err := os.ReadFile(filepath.Join("..", "..", "auth-defaults", "site.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(theirs, builtIn) {
		t.Error("auth-defaults/site.json and anahita-libs/about/default.json differ: copy one over the other")
	}
}

func TestTheBuiltInSiteIsWhole(t *testing.T) {
	site := Default()
	if !site.valid() || site.Rules.Version == "" || len(site.Rules.Items) != 6 || len(site.Promises) != 3 || len(site.RuleSuggestions) != 8 {
		t.Fatalf("%+v", site)
	}
	for _, language := range []string{"en-CA", "fr-FR", "de-DE", "es-419", "pt-BR"} {
		for _, item := range append(append([]Item{}, site.Rules.Items...), site.Promises...) {
			if item.Title[language] == "" || item.Body[language] == "" {
				t.Errorf("%q lacks %s", item.Title["en-CA"], language)
			}
		}
		for _, item := range site.RuleSuggestions {
			if item.Title[language] == "" {
				t.Errorf("%q lacks %s", item.Title["en-CA"], language)
			}
		}
	}
}

func TestWordsInTheNearestLanguage(t *testing.T) {
	words := Text{"en-CA": "Hello", "fr-FR": "Bonjour"}
	for language, want := range map[string]string{"fr-FR": "Bonjour", "fr-CA": "Bonjour", "en-US": "Hello", "de-DE": "Hello"} {
		if got := words.In(language); got != want {
			t.Errorf("%s: %q", language, got)
		}
	}
}

func TestWhatAFileSays(t *testing.T) {
	// Only promises: the rest as it comes.
	site, ok := Parse([]byte(`{"promises":[{"title":{"en-CA":"Daily backups."}}]}`))
	if !ok || len(site.Promises) != 1 || len(site.Rules.Items) != 6 {
		t.Errorf("%v %+v", ok, site)
	}
	// No rules at all is an installation's choice.
	if site, ok := Parse([]byte(`{"rules":{"items":[]}}`)); !ok || len(site.Rules.Items) != 0 {
		t.Errorf("no rules: %v %+v", ok, site.Rules)
	}
	for name, raw := range map[string]string{
		"not JSON":           `{`,
		"no version":         `{"rules":{"items":[{"title":{"en-CA":"Be kind."}}]}}`,
		"a version of words": `{"rules":{"version":"v2","items":[{"title":{"en-CA":"Be kind."}}]}}`,
		"an empty title":     `{"promises":[{"title":{}}]}`,
		"eleven rules":       `{"rules":{"version":"1","items":[` + repeat(`{"title":{"en-CA":"R"}}`, 11) + `]}}`,
	} {
		if site, ok := Parse([]byte(raw)); ok || len(site.Rules.Items) != 6 {
			t.Errorf("%s was taken", name)
		}
	}
}

func repeat(item string, n int) string {
	out := item
	for i := 1; i < n; i++ {
		out += "," + item
	}
	return out
}

func TestDaysAreTheConfigurations(t *testing.T) {
	site := Default().Filled(14)
	if got := site.Promises[0].Body.In("en-CA"); got != "You have 14 days to undo deleting your account. Just sign in again." {
		t.Errorf("%q", got)
	}
}

func TestTheFileIsReadAgainWhenItChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "site.json")
	source := NewSource(path)
	if len(source.Site().Promises) != 3 {
		t.Fatal("with no file, not as it comes")
	}
	write := func(raw string, at time.Time) {
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(path, at, at)
	}
	now := time.Now()
	write(`{"promises":[]}`, now)
	if len(source.Site().Promises) != 0 {
		t.Error("the file was not read")
	}
	write(`{"promises":[{"title":{"en-CA":"Daily backups."}}]}`, now.Add(time.Second))
	if len(source.Site().Promises) != 1 {
		t.Error("the change was not read")
	}
}
