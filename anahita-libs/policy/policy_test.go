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

func TestSiteReadAccessIsReadStrictly(t *testing.T) {
	for raw, want := range map[string]string{
		"":            ReadPublic,
		"public":      ReadPublic,
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
