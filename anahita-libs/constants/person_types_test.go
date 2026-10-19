package constants

import "testing"

// Who may change a person's type, from what to what (the user, 2026-10-18):
// an administrator makes administrators of registered people; everything
// else is a super administrator's.
func TestMayChangePersonType(t *testing.T) {
	const (
		reg   = PersonTypeRegistered
		admin = PersonTypeAdmin
		super = PersonTypeSuperAdmin
	)
	for _, each := range []struct {
		viewer, from, to string
		may              bool
	}{
		{admin, reg, admin, true},
		{admin, reg, super, false},
		{admin, admin, reg, false},
		{admin, admin, super, false},
		{admin, super, admin, false},
		{admin, super, reg, false},
		{super, reg, admin, true},
		{super, reg, super, true},
		{super, admin, reg, true},
		{super, admin, super, true},
		{super, super, admin, true},
		{super, super, reg, true},
		{reg, reg, admin, false},
		{PersonTypeGuest, reg, admin, false},
		{super, reg, reg, false},
		{super, reg, PersonTypeGuest, false},
		{super, "", admin, false},
	} {
		if got := MayChangePersonType(each.viewer, each.from, each.to); got != each.may {
			t.Errorf("%s changing %s to %s: %v; want %v", each.viewer, each.from, each.to, got, each.may)
		}
	}
}
