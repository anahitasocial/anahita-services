package requests

import (
	"reflect"
	"strings"
	"testing"
)

// Signup and invite-accept create the same thing by two routes, and they had
// drifted: six characters on the public path anyone can reach, fifteen on the
// invited one; a name required on one and not collected at all on the other.
//
// The asymmetry ran the wrong way, which is what makes it worth a test rather
// than a comment. These read the struct tags so the two cannot drift again
// without something saying so.

func tagOf(t *testing.T, v any, field, key string) string {
	t.Helper()

	f, ok := reflect.TypeOf(v).Elem().FieldByName(field)
	if !ok {
		t.Fatalf("%T has no field %s", v, field)
	}

	return f.Tag.Get(key)
}

func TestAccountCreationPathsShareTheirRules(t *testing.T) {
	signup := &SignupAddRequest{}
	invite := &InviteAccept{}

	for _, field := range []string{"Password", "Name"} {
		t.Run(field, func(t *testing.T) {
			s := tagOf(t, signup, field, "validate")
			i := tagOf(t, invite, field, "validate")

			if s != i {
				t.Errorf("signup validates %s as %q, invite-accept as %q. Both "+
					"create an account; a rule that holds on one path and not "+
					"the other is a rule that does not hold",
					field, s, i)
			}
		})
	}
}

// Fifteen is the defensible number and six was the outlier. Pinned as a value
// rather than only as "the same on both paths", so a future edit cannot settle
// the disagreement by lowering both.
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

// `required` on a bool passes an unticked box straight through: false IS the
// zero value, and that is what `required` tests for. The whole point of the
// checkbox is that it must be ticked.
func TestAgreementCheckboxesCannotBeLeftUnticked(t *testing.T) {
	for name, req := range map[string]any{
		"signup":        &SignupAddRequest{},
		"invite-accept": &InviteAccept{},
	} {
		for _, field := range []string{"TosAccepted", "PpAccepted"} {
			t.Run(name+"/"+field, func(t *testing.T) {
				tag := tagOf(t, req, field, "validate")

				if tag != "eq=true" {
					t.Errorf("%s is validated as %q. `required` on a bool is "+
						"satisfied by false, so an unticked box would create "+
						"an account claiming to have accepted the terms", field, tag)
				}
			})
		}
	}
}

// Signup must collect a display name. nodes.name is THE name now, and a
// Go-created account used to land with none at all — the 823 legacy people
// only have one because they predate this path.
func TestSignupCollectsADisplayName(t *testing.T) {
	tag := tagOf(t, &SignupAddRequest{}, "Name", "validate")

	if !strings.Contains(tag, "required") {
		t.Errorf("Name is validated as %q; signup must collect one", tag)
	}

	// alphanumunicode is ^[\p{L}\p{N}]+$ — it rejects the space between two
	// words, along with Jean-Luc, O'Brien and van der Berg.
	if strings.Contains(tag, "alphanumunicode") {
		t.Errorf("Name is validated as %q, which rejects any name containing "+
			"a space, a hyphen or an apostrophe", tag)
	}
}
