package repositories

import (
	"strings"
	"testing"
)

// The search term is bound as a parameter, so this is not SQL injection — the
// risk is LIKE-pattern injection. The term is wrapped as "%term%" and handed
// to LIKE, where % and _ are wildcards, so a term containing them changes what
// the pattern MATCHES rather than what statement runs.
//
// A single "%" would match every row in the table on both name and body, which
// is a cheap way to make the database scan everything on demand.

func TestSanitizeSearchStripsLikeWildcards(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"ordinary term", "lovelace", "lovelace"},
		{"trims surrounding space", "  lovelace  ", "lovelace"},

		// The wildcards. Stripped rather than escaped, so the term matches
		// literally and no pattern the caller writes can widen it.
		{"percent alone matches everything", "%", ""},
		{"underscore matches any character", "_", ""},
		{"percent inside a word", "ada%lovelace", "adalovelace"},
		{"underscore inside a word", "ada_lovelace", "adalovelace"},
		{"a pattern that would match every row", "%%%", ""},
		{"mixed", "%_ada_%", "ada"},

		// The backslash is MySQL's default LIKE escape character, so a term
		// ending in one escapes the closing "%" of the pattern wrapped around
		// it: "%ada\\%" looks for a literal percent sign and finds nothing.
		{"trailing backslash", `ada\`, "ada"},
		{"backslash inside a word", `ada\lovelace`, "adalovelace"},

		// Not wildcards, and must survive: these are ordinary characters in a
		// name or a bio.
		{"apostrophe", "O'Brien", "O'Brien"},
		{"quote", `say "hi"`, `say "hi"`},
		{"unicode", "Ada Lovelacé", "Ada Lovelacé"},
		{"emoji", "cycling 🚲", "cycling 🚲"},
		{"hyphen", "jean-luc", "jean-luc"},

		{"empty", "", ""},
		{"only whitespace", "   ", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeSearchString(tc.input); got != tc.want {
				t.Errorf("sanitizeSearchString(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// The property that matters, stated directly: whatever comes out must contain
// no wildcard, for any input.
func TestSanitizedSearchNeverContainsAWildcard(t *testing.T) {
	inputs := []string{
		"%", "_", "%%", "__", "%_%", "a%b_c",
		"100% cotton", "first_name", "%'; DROP TABLE nodes; --",
		strings.Repeat("%", 100),
	}

	for _, input := range inputs {
		got := sanitizeSearchString(input)
		if strings.ContainsAny(got, "%_") {
			t.Errorf("sanitizeSearchString(%q) = %q, which still carries a LIKE wildcard",
				input, got)
		}
	}
}

// A term that sanitises to nothing must not become an empty LIKE. The callers
// guard on `req.Search != ""` after sanitising — if that check were moved
// before the sanitise, a search for "%" would build "%%" and match every row.
func TestATermOfOnlyWildcardsSanitisesToEmpty(t *testing.T) {
	for _, input := range []string{"%", "%%", "_", "__", "%_", " % "} {
		if got := sanitizeSearchString(input); got != "" {
			t.Errorf("sanitizeSearchString(%q) = %q; the caller's non-empty check "+
				"would then build a pattern matching every row", input, got)
		}
	}
}
