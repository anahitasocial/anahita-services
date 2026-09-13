package requests

import (
	"reflect"
	"strings"
	"testing"
)

// Signup and invite-accept create the same thing by two routes, and they had
// drifted: six characters on the public path anyone can reach, fifteen on the
// invited one. The asymmetry ran the wrong way, which is what makes it worth a
// test rather than a comment.
//
// What they share and what they differ on is now deliberate, so both are
// pinned here.

func tagOf(t *testing.T, v any, field, key string) string {
	t.Helper()

	f, ok := reflect.TypeOf(v).Elem().FieldByName(field)
	if !ok {
		t.Fatalf("%T has no field %s", v, field)
	}

	return f.Tag.Get(key)
}

func hasField(v any, field string) bool {
	_, ok := reflect.TypeOf(v).Elem().FieldByName(field)
	return ok
}

// Fifteen on every path. Pinned as a value rather than only as "the same on
// both", so a future edit cannot settle the disagreement by lowering both.
func TestAccountCreationRequiresAStrongPassword(t *testing.T) {
	for name, req := range map[string]any{
		"signup":         &SignupAddRequest{},
		"invite-accept":  &InviteAccept{},
		"password reset": &PasswordReset{},
	} {
		t.Run(name, func(t *testing.T) {
			if tag := tagOf(t, req, "Password", "validate"); !strings.Contains(tag, "min=15") {
				t.Errorf("password rule is %q, want min=15", tag)
			}
		})
	}
}

// Signup asks for a handle, an email and a password, and nothing else.
//
// Neither Mastodon nor Bluesky asks for a display name, and every field added
// here is a field somebody has to get past to join. The profile is onboarding's
// job, where it is skippable and the editor is better than a plain form.
func TestSignupAsksForTheMinimum(t *testing.T) {
	req := &SignupAddRequest{}

	for _, required := range []string{"Email", "Username", "Password"} {
		if !hasField(req, required) {
			t.Errorf("signup no longer collects %s", required)
		}
	}

	for _, gone := range []string{
		"Name", "Body", "WebsiteURL", "PersonPronouns",
		"TosAccepted", "TosVersion", "PpAccepted", "PpVersion",
	} {
		if hasField(req, gone) {
			t.Errorf("signup collects %s. The minimum is a handle, an email "+
				"and a password; a profile is onboarding's job and the terms "+
				"are accepted after signing in", gone)
		}
	}
}

// The form is CSRF-bound, which is half the reason signup is server-rendered
// at all. A bound token cannot be carried by a form an attacker hosts.
func TestSignupIsCSRFBound(t *testing.T) {
	tag := tagOf(t, &SignupAddRequest{}, "CSRFToken", "validate")

	if !strings.Contains(tag, "required") {
		t.Errorf("CSRFToken is validated as %q; an absent token must not pass", tag)
	}
}

// A name is offered on the invited path and required nowhere.
//
// Somebody vouched for an invitee, so there is nothing a required name would
// support — nobody is judging the answer. Requiring it mostly produces names
// typed to get past the field.
func TestNoPathRequiresADisplayName(t *testing.T) {
	tag := tagOf(t, &InviteAccept{}, "Name", "validate")

	if strings.Contains(tag, "required") {
		t.Errorf("invite-accept requires a display name (%q). An empty one "+
			"falls back to the handle, which is what both federation "+
			"protocols do", tag)
	}

	// alphanumunicode is ^[\p{L}\p{N}]+$ — it rejects the space between two
	// words, along with Jean-Luc, O'Brien and van der Berg.
	if strings.Contains(tag, "alphanumunicode") {
		t.Errorf("Name is validated as %q, which rejects any name containing "+
			"a space, a hyphen or an apostrophe", tag)
	}
}

// `required` on a bool is satisfied by false, which is an unticked box. Still
// asserted where checkboxes remain — invite-accept and the set-password page
// keep collecting until the post-login interstitial replaces them, and
// removing them before it exists would leave nothing collecting anything.
func TestRemainingAgreementCheckboxesCannotBeLeftUnticked(t *testing.T) {
	req := &InviteAccept{}

	for _, field := range []string{"TosAccepted", "PpAccepted"} {
		t.Run(field, func(t *testing.T) {
			if tag := tagOf(t, req, field, "validate"); tag != "eq=true" {
				t.Errorf("%s is validated as %q. `required` on a bool is "+
					"satisfied by false, so an unticked box would create an "+
					"account claiming to have accepted the terms", field, tag)
			}
		})
	}
}
