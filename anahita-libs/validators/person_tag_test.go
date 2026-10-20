package validators

import (
	"testing"

	"anahita-libs/requests"

	"github.com/go-playground/validator/v10"
)

// Who may tag a person: anybody signed in, each approved first, or nobody
// (roadmap row 103). Anything else would refuse everybody without a word.
func TestWhoMayTagIsOneOfThree(t *testing.T) {
	rv := NewRequestValidator(validator.New())
	for access, want := range map[string]bool{
		"registered": true,
		"approve":    true,
		"admins":     true,
		"followers":  false,
		"public":     false,
	} {
		err := rv.Validate(requests.PersonEditAddPermission{Entity: "tag", Access: access})
		if (err == nil) != want {
			t.Errorf("%s: %v, want valid %v", access, err, want)
		}
	}
}
