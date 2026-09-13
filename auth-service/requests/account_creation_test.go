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

// No account-creation path collects agreement any more.
//
// Acceptance is asked for after signing in and checked on every sign-in, so a
// revision re-prompts by itself. A path that also collected at the door would
// mean two places writing the same record, and a form that grows a field back
// every time somebody worries about the gap.
//
// The set-password page is the one exception left, and it is deliberate: the
// seeded super administrator reaches it before they have ever signed in. It
// goes when the interstitial lands.
func TestAccountCreationDoesNotCollectAgreement(t *testing.T) {
	for name, req := range map[string]any{
		"signup":        &SignupAddRequest{},
		"invite-accept": &InviteAccept{},
	} {
		for _, field := range []string{"TosAccepted", "TosVersion", "PpAccepted", "PpVersion"} {
			t.Run(name+"/"+field, func(t *testing.T) {
				if hasField(req, field) {
					t.Errorf("%s still collects %s. Acceptance belongs in one "+
						"place, and that place is after sign-in where a "+
						"revision can re-prompt", name, field)
				}
			})
		}
	}
}

// Both paths ask for a username, because it is the account's address and not a
// profile field. Invite-accept derived one from the email and never showed it,
// which meant two people invited as alice@ at different domains collided and
// the second was told to try again — forever.
func TestBothPathsChooseAUsername(t *testing.T) {
	for name, req := range map[string]any{
		"signup":        &SignupAddRequest{},
		"invite-accept": &InviteAccept{},
	} {
		t.Run(name, func(t *testing.T) {
			tag := tagOf(t, req, "Username", "validate")

			for _, rule := range []string{"required", "username", "min=3", "max=32"} {
				if !strings.Contains(tag, rule) {
					t.Errorf("username is validated as %q, missing %q — the two "+
						"paths must agree, or one offers a name the other refuses",
						tag, rule)
				}
			}
		})
	}
}
