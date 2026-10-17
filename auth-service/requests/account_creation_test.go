package requests

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"anahita-libs/constants"
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

// Signup asks for a username, an email and a password, and — only under
// `approval` — the three fields a human needs to decide on.
//
// Neither Mastodon nor Bluesky asks for a display name, and every field added
// is one more thing between somebody and joining. Which is why the profile
// fields are OPTIONAL in the struct and required by the handler: the same form
// binds both modes, and only the handler knows which one is running.
func TestSignupAsksForTheMinimum(t *testing.T) {
	req := &SignupAddRequest{}

	for _, required := range []string{"Email", "Username", "Password"} {
		if !hasField(req, required) {
			t.Errorf("signup no longer collects %s", required)
		}
	}

	// Present for the approval form, and never required by the tag — an
	// `open` signup that had to fill them in would be the opposite of the
	// decision this whole item rests on.
	for _, field := range []string{"Name", "Body", "WebsiteURL"} {
		tag := tagOf(t, req, field, "validate")
		if strings.Contains(tag, "required") {
			t.Errorf("%s is validated as %q. It is collected only under "+
				"`approval`, so requiring it in the tag would demand it in "+
				"every mode", field, tag)
		}
	}

	for _, gone := range []string{
		"PersonPronouns",
		"TosAccepted", "TosVersion", "PpAccepted", "PpVersion",
	} {
		if hasField(req, gone) {
			t.Errorf("signup collects %s. Pronouns belong to onboarding and "+
				"the terms are accepted after signing in", gone)
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

			for _, rule := range []string{"required", "username",
				fmt.Sprintf("min=%d", constants.UsernameMin),
				fmt.Sprintf("max=%d", constants.UsernameMax)} {
				if !strings.Contains(tag, rule) {
					t.Errorf("username is validated as %q, missing %q — the two "+
						"paths must agree, or one offers a name the other refuses",
						tag, rule)
				}
			}
		})
	}
}
