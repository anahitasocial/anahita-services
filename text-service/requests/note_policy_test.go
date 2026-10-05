package requests

import (
	"testing"

	"anahita-libs/policy"
)

// A note written without saying who can see it: public, or registered on an
// installation that allows nothing new to be public.
func TestANoteDefaultsToWhatTheInstallationAllows(t *testing.T) {
	if got := NewNoteRequest().Add.Access; got != "public" {
		t.Errorf("a new note defaults to %q on an open installation", got)
	}

	restore := policy.SetForTest("registered")
	defer restore()

	if got := NewNoteRequest().Add.Access; got != "registered" {
		t.Errorf("a new note defaults to %q where nothing new may be public", got)
	}
	if got := NewNoteRequest().EditAccess.Access; got != "registered" {
		t.Errorf("a changed note defaults to %q", got)
	}
}
