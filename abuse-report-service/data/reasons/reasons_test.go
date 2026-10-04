package reasons

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTheBuiltInListIsValidAndComplete(t *testing.T) {
	catalogue := BuiltIn()

	if len(catalogue.Reasons) < 10 {
		t.Fatalf("%d reasons", len(catalogue.Reasons))
	}

	for _, reason := range catalogue.Reasons {
		for _, language := range []string{"en-GB", "fr-FR"} {
			if reason.Label[language] == "" || reason.Description[language] == "" {
				t.Errorf("%s has no %s label or description", reason.Key, language)
			}
		}
	}

	other, ok := catalogue.Find("other")
	if !ok || !other.RequiresDetails {
		t.Error(`"other" must exist and require an explanation`)
	}
}

func TestReasonsApplyByTypePrefix(t *testing.T) {
	catalogue := BuiltIn()

	keys := func(nodeType string) map[string]bool {
		out := map[string]bool{}
		for _, reason := range catalogue.For(nodeType) {
			out[reason.Key] = true
		}
		return out
	}

	person := keys("node.actor.person-service.person.v1")
	note := keys("node.medium.text-service.note.v1")
	hashtag := keys("node.tag.hashtag-service.hashtag.v1")

	if !person["impersonation"] {
		t.Error("a person cannot be reported for impersonation")
	}
	if note["impersonation"] || hashtag["impersonation"] {
		t.Error("a post or a hashtag can be reported for impersonation")
	}
	for _, kind := range []map[string]bool{person, note, hashtag} {
		if !kind["spam"] || !kind["other"] {
			t.Error("spam and other must apply to everything")
		}
	}
}

func TestLabelsFallBackToARelatedLanguageThenTheDefault(t *testing.T) {
	reason, _ := BuiltIn().Find("hate_racism")

	if got := reason.LabelIn("fr-FR"); got != "Racisme" {
		t.Errorf("fr-FR: %q", got)
	}
	if got := reason.LabelIn("fr-CA"); got != "Racisme" {
		t.Errorf("fr-CA should read the French label, got %q", got)
	}
	if got := reason.LabelIn("de-DE"); got != "Racism" {
		t.Errorf("an unoffered language should read the default, got %q", got)
	}
	if got := reason.LabelIn(""); got != "Racism" {
		t.Errorf("no language should read the default, got %q", got)
	}
}

func TestParseRefusesAnUnusableFile(t *testing.T) {
	cases := map[string]string{
		"not JSON":               `{"reasons": [`,
		"no reasons":             `{"reasons": []}`,
		"a missing key":          `{"reasons": [{"label": {"en-GB": "Spam"}}]}`,
		"a repeated key":         `{"reasons": [{"key": "a", "label": {"en-GB": "A"}}, {"key": "a", "label": {"en-GB": "B"}}]}`,
		"no default label":       `{"reasons": [{"key": "a", "label": {"fr-FR": "A"}}]}`,
		"a key that is too long": `{"reasons": [{"key": "` + string(make([]byte, 0)) + `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "label": {"en-GB": "A"}}]}`,
	}

	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(data)); err == nil {
				t.Error("accepted")
			}
		})
	}
}

func write(t *testing.T, path, data string, at time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestSourceWithoutAPathUsesTheBuiltInList(t *testing.T) {
	if got := len(NewSource("").Catalogue().Reasons); got != len(BuiltIn().Reasons) {
		t.Errorf("%d reasons", got)
	}
}

// An installation's file is used, picked up again when edited, and replaced
// by the built-in list for as long as it is broken or missing.
func TestSourceFollowsTheFileAndFallsBackWhenItBreaks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reasons.json")
	start := time.Now().Add(-time.Hour)

	write(t, path, `{"reasons": [{"key": "rude", "label": {"en-GB": "Rude"}}]}`, start)
	source := NewSource(path)

	if got := source.Catalogue().Reasons; len(got) != 1 || got[0].Key != "rude" {
		t.Fatalf("the file was not used: %v", got)
	}

	write(t, path, `{"reasons": [{"key": "rude", "label": {"en-GB": "Rude"}}, {"key": "loud", "label": {"en-GB": "Loud"}}]}`,
		start.Add(time.Minute))
	if got := len(source.Catalogue().Reasons); got != 2 {
		t.Fatalf("an edit was not picked up: %d reasons", got)
	}

	write(t, path, `{"reasons": [`, start.Add(2*time.Minute))
	if got := len(source.Catalogue().Reasons); got != len(BuiltIn().Reasons) {
		t.Fatalf("a broken file was not replaced by the built-in list: %d reasons", got)
	}

	write(t, path, `{"reasons": [{"key": "fixed", "label": {"en-GB": "Fixed"}}]}`, start.Add(3*time.Minute))
	if got := source.Catalogue().Reasons; len(got) != 1 || got[0].Key != "fixed" {
		t.Fatalf("the repaired file was not picked up: %v", got)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := len(source.Catalogue().Reasons); got != len(BuiltIn().Reasons) {
		t.Fatalf("a missing file was not replaced by the built-in list: %d reasons", got)
	}
}
