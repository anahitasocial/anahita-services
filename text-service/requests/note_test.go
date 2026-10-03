package requests

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

// A note's access is one keyword. It is never a list of person ids, which is
// how the PHP app stored a private note between two people; the Go services
// have no private messaging, and the rows that had one were deleted
// (migrations/20261003000001_delete_private_notes.sql). This keeps the shape
// from coming back through the API.
func TestNoteAccessIsAKeyword(t *testing.T) {
	validate := validator.New()

	cases := []struct {
		access string
		ok     bool
	}{
		{"public", true},
		{"registered", true},
		{"followers", true},
		{"leaders", true},
		{"mutuals", true},
		{"admins", true},
		// "Only me". The validator used to accept `owner` here, a value no
		// other post type knows and the web app never sends, so choosing
		// "Only me" for a note was refused.
		{"myself", true},
		{"owner", false},

		{"21010,1", false},
		{"5", false},
		{"public,5", false},
		{"private", false},
	}

	for _, c := range cases {
		t.Run(c.access, func(t *testing.T) {
			add := validate.Struct(&NoteAddRequest{OwnerID: 1, Body: "hello", Access: c.access})
			edit := validate.Struct(&NoteEditAccessRequest{ID: 1, Access: c.access})

			if (add == nil) != c.ok {
				t.Errorf("adding a note with access %q: err = %v, want accepted = %v", c.access, add, c.ok)
			}
			if (edit == nil) != c.ok {
				t.Errorf("changing access to %q: err = %v, want accepted = %v", c.access, edit, c.ok)
			}
		})
	}

	// Leaving it out is allowed when adding: the note takes the default.
	if err := validate.Struct(&NoteAddRequest{OwnerID: 1, Body: "hello"}); err != nil {
		t.Errorf("adding a note with no access: %v", err)
	}
}
