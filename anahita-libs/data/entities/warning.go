package entities

import (
	"strings"
	"unicode/utf8"
)

// A post's content warning (anahita-services row 122,
// docs/plans/content-warnings.md): what it holds, so that each reader can
// choose to see it at once, behind a warning, or not at all. The first four
// are Bluesky's self-labels, one for one; Other is a warning in the
// author's own words, as the fediverse writes them.
const (
	WarningAdult      = "adult"
	WarningSuggestive = "suggestive"
	WarningGraphic    = "graphic"
	WarningNudity     = "nudity"
	WarningOther      = "other"

	// WarningTextMax is how long an Other warning's words may be, in
	// characters.
	WarningTextMax = 100
)

// WarningKinds are the kinds a post may be given, in the order the composer
// lists them.
var WarningKinds = []string{
	WarningAdult,
	WarningSuggestive,
	WarningGraphic,
	WarningNudity,
	WarningOther,
}

// IsWarningKind says whether kind is one of the kinds, the empty string (no
// warning) not among them.
func IsWarningKind(kind string) bool {
	for _, k := range WarningKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// NormalizeWarning checks a warning as a request gives it, and returns what
// is kept: no kind is no warning, and words are kept for Other alone. ok is
// false for a kind that is not one, and for an Other with no words or more
// than WarningTextMax characters.
func NormalizeWarning(kind, text string) (string, string, bool) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return "", "", true
	}
	if !IsWarningKind(kind) {
		return "", "", false
	}
	if kind != WarningOther {
		return kind, "", true
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || utf8.RuneCountInString(text) > WarningTextMax {
		return "", "", false
	}
	return kind, text, true
}

// The self-label each kind is on Bluesky, one for one; Other has none: a
// warning in one's own words is not something Bluesky has.
var warningLabels = map[string]string{
	WarningAdult:      "porn",
	WarningSuggestive: "sexual",
	WarningGraphic:    "graphic-media",
	WarningNudity:     "nudity",
}

// WarningLabel is the self-label a post with this warning is sent to
// Bluesky with, or "" for none.
func WarningLabel(kind string) string {
	return warningLabels[kind]
}

// The words of each kind, as the fediverse is told them: in English, as
// the web app says them in en-CA, since one post goes to every server.
var warningWords = map[string]string{
	WarningAdult:      "Adult content",
	WarningSuggestive: "Sexually suggestive",
	WarningGraphic:    "Graphic media",
	WarningNudity:     "Non-sexual nudity",
}

// WarningSaid is a warning as the fediverse is told it (a post's summary):
// its author's words for Other, its kind's otherwise, or "" for none.
func WarningSaid(kind, text string) string {
	if kind == WarningOther {
		return strings.TrimSpace(text)
	}
	return warningWords[kind]
}
