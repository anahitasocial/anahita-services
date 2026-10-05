package policy

import (
	"fmt"
	"testing"
)

func TestMinContentAccessIsReadStrictly(t *testing.T) {
	for raw, want := range map[string]string{
		"":             "public",
		"public":       "public",
		" Public ":     "public",
		"registered":   "registered",
		"REGISTERED":   "registered",
		"members":      "registered", // not understood: the stricter one
		"privat":       "registered",
		"followers":    "registered",
		"public,other": "registered",
	} {
		if got := readMinContentAccess(raw); got != want {
			t.Errorf("%q read as %q, want %q", raw, got, want)
		}
	}
}

func TestOnlyPublicIsEverRuledOut(t *testing.T) {
	levels := []string{"public", "registered", "followers", "myself"}

	restore := SetForTest("registered")
	if got := fmt.Sprint(Allowed(levels)); got != "[registered followers myself]" {
		t.Errorf("with no public content allowed: %s", got)
	}
	if AllowsPublic() || AllowsAccess("public") || !AllowsAccess("followers") {
		t.Error("the registered setting does not rule out public alone")
	}
	restore()

	if got := fmt.Sprint(Allowed(levels)); got != "[public registered followers myself]" {
		t.Errorf("by default: %s", got)
	}
	if MinContentAccess() != "public" {
		t.Errorf("the setting was not put back: %q", MinContentAccess())
	}
}

// What is already public may stay public; nothing may become public.
func TestAccessThatIsAlreadyThereMayStay(t *testing.T) {
	restore := SetForTest("registered")
	defer restore()

	for _, tc := range []struct {
		current, requested string
		want               bool
	}{
		{"public", "public", true},      // unchanged
		{"public", "followers", true},   // narrowed
		{"followers", "public", false},  // widened to public
		{"registered", "public", false}, // likewise
		{"", "public", false},           // never said, now public
		{"followers", "registered", true},
	} {
		if got := MayChangeAccess(tc.current, tc.requested); got != tc.want {
			t.Errorf("from %q to %q: %v, want %v", tc.current, tc.requested, got, tc.want)
		}
	}

	restore()
	if !MayChangeAccess("followers", "public") {
		t.Error("an open installation refused a change to public")
	}
}

// The three values of SITE_READ_ACCESS are told apart, and only one of the
// two restrictions is ever on.
func TestTheThreeKindsOfSite(t *testing.T) {
	for value, want := range map[string][2]bool{
		"public":     {false, false},
		"preview":    {false, true},
		"registered": {true, false},
	} {
		restore := SetReadAccessForTest(value)
		if MembersOnly() != want[0] || PreviewOnly() != want[1] {
			t.Errorf("%s: members-only=%v preview=%v", value, MembersOnly(), PreviewOnly())
		}
		restore()
	}
}

func TestSiteReadAccessIsReadStrictly(t *testing.T) {
	for raw, want := range map[string]string{
		"":            ReadPublic,
		"public":      ReadPublic,
		"preview":     ReadPreview,
		"registered":  ReadRegistered,
		" Registered": ReadRegistered,
		"members":     ReadRegistered, // not understood: the stricter one
		"private":     ReadRegistered,
	} {
		if got := readSiteReadAccess(raw); got != want {
			t.Errorf("%q read as %q, want %q", raw, got, want)
		}
	}

	restore := SetReadAccessForTest("registered")
	if !MembersOnly() {
		t.Error("registered is not members-only")
	}
	restore()
	if MembersOnly() {
		t.Error("the setting was not put back")
	}
}
