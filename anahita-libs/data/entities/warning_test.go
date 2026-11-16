package entities

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// THE RULE THIS GUARDS: a post from Bluesky takes the kind of its
// strongest self-label, one for one with what is sent there.
func TestWarningFromLabels(t *testing.T) {
	cases := []struct {
		labels []string
		want   string
	}{
		{nil, ""},
		{[]string{"!no-unauthenticated"}, ""},
		{[]string{"porn"}, WarningAdult},
		{[]string{"sexual"}, WarningSuggestive},
		{[]string{"nudity"}, WarningNudity},
		{[]string{"graphic-media"}, WarningGraphic},
		{[]string{"gore"}, WarningGraphic},
		{[]string{"nudity", "sexual", "porn"}, WarningAdult},
		{[]string{"nudity", "graphic-media"}, WarningGraphic},
	}
	for _, c := range cases {
		if got := WarningFromLabels(c.labels); got != c.want {
			t.Errorf("%v: got %q, want %q", c.labels, got, c.want)
		}
	}
	// What is sent there is read back as itself.
	for _, kind := range []string{WarningAdult, WarningSuggestive, WarningGraphic, WarningNudity} {
		if got := WarningFromLabels([]string{WarningLabel(kind)}); got != kind {
			t.Errorf("%s sent as %s read back as %q", kind, WarningLabel(kind), got)
		}
	}
}

// THE RULE THIS GUARDS: a post from the fediverse is warned of in its
// author's words, or as Sensitive when it is marked with none, and never
// with words longer than any warning here.
func TestWarningFromFediverse(t *testing.T) {
	if kind, text := WarningFromFediverse(false, ""); kind != "" || text != "" {
		t.Errorf("plain: %q %q", kind, text)
	}
	if kind, text := WarningFromFediverse(true, "  "); kind != WarningOther || text != WarningSensitive {
		t.Errorf("sensitive alone: %q %q", kind, text)
	}
	if kind, text := WarningFromFediverse(true, "  Spoilers:\n the  ending "); kind != WarningOther || text != "Spoilers: the ending" {
		t.Errorf("with words: %q %q", kind, text)
	}
	if kind, text := WarningFromFediverse(false, "Eye contact"); kind != WarningOther || text != "Eye contact" {
		t.Errorf("words without sensitive: %q %q", kind, text)
	}
	kind, text := WarningFromFediverse(true, strings.Repeat("long ", 40))
	if kind != WarningOther || utf8.RuneCountInString(text) > WarningTextMax || !strings.HasSuffix(text, "…") {
		t.Errorf("long: %q %q (%d)", kind, text, utf8.RuneCountInString(text))
	}
	if _, _, ok := NormalizeWarning(kind, text); !ok {
		t.Errorf("a long one cut is still a warning here: %q", text)
	}
}

// THE RULE THIS GUARDS: every kind starts at Warn, a reader's own choice
// is kept, and only what differs from Warn is stored.
func TestWarningChoices(t *testing.T) {
	got := WarningChoices(map[string]string{
		WarningGraphic: WarningHide,
		WarningNudity:  WarningShow,
		WarningAdult:   "maybe",
		"made-up":      WarningShow,
	})
	want := map[string]string{
		WarningAdult:      WarningWarn,
		WarningSuggestive: WarningWarn,
		WarningGraphic:    WarningHide,
		WarningNudity:     WarningShow,
		WarningOther:      WarningWarn,
		WarningAI:         WarningShow,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for kind, choice := range want {
		if got[kind] != choice {
			t.Errorf("%s: got %q, want %q", kind, got[kind], choice)
		}
	}

	kept := WarningChoicesKept(map[string]string{
		WarningGraphic: WarningHide,
		WarningAdult:   WarningWarn,
		WarningAI:      WarningShow,
	})
	if len(kept) != 1 || kept[WarningGraphic] != WarningHide {
		t.Errorf("kept %v, want graphic: hide alone", kept)
	}
	if kept := WarningChoicesKept(map[string]string{WarningAI: WarningWarn}); kept[WarningAI] != WarningWarn {
		t.Errorf("made with AI at Warn was not kept: %v", kept)
	}
	if WarningHides(WarningAI) || WarningHides("") || !WarningHides(WarningOther) {
		t.Error("only a warning of what a post holds hides it")
	}
}
