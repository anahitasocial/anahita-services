package requests

import (
	"testing"

	"anahita-libs/policy"
)

// What a post or a profile gets when the client does not say who can see it:
// public, or registered on an installation that allows nothing new to be
// public. Read when the request is made, not when the program starts.
func TestDefaultsFollowTheInstallation(t *testing.T) {
	if got := NewMediumRequest().Add.Access; got != "public" {
		t.Errorf("a new post defaults to %q on an open installation", got)
	}

	restore := policy.SetForTest("registered")
	defer restore()

	if got := NewMediumRequest().Add.Access; got != "registered" {
		t.Errorf("a new post defaults to %q where nothing new may be public", got)
	}
	if got := NewMediumRequest().EditAccess.Access; got != "registered" {
		t.Errorf("a changed post defaults to %q", got)
	}
	if got := NewPersonRequest().EditAccess.Access; got != "registered" {
		t.Errorf("a profile's access defaults to %q", got)
	}
}
