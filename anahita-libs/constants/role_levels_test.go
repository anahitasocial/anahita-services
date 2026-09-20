package constants

import "testing"

// The scale two settings share — INVITES_FROM and GROUPS_FROM.
//
// The cases that matter are the edges, because the middle is obvious and the
// edges are where this has been wrong before: a single ranking function sent
// guests to the top of the scale, and every guest passed under `registered`.

func TestMeetsRoleLevelAdmitsFromTheLevelUpwards(t *testing.T) {
	cases := []struct {
		personType string
		level      string
		want       bool
	}{
		{PersonTypeRegistered, RoleLevelRegistered, true},
		{PersonTypeAdmin, RoleLevelRegistered, true},
		{PersonTypeSuperAdmin, RoleLevelRegistered, true},

		{PersonTypeRegistered, RoleLevelAdministrators, false},
		{PersonTypeAdmin, RoleLevelAdministrators, true},
		{PersonTypeSuperAdmin, RoleLevelAdministrators, true},

		{PersonTypeRegistered, RoleLevelSuperAdministrators, false},
		{PersonTypeAdmin, RoleLevelSuperAdministrators, false},
		{PersonTypeSuperAdmin, RoleLevelSuperAdministrators, true},
	}

	for _, tc := range cases {
		if got := MeetsRoleLevel(tc.personType, tc.level); got != tc.want {
			t.Errorf("MeetsRoleLevel(%q, %q) = %v, want %v",
				tc.personType, tc.level, got, tc.want)
		}
	}
}

// The regression that produced the two scales. A guest is not the plural of
// any level and must clear none of them.
func TestMeetsRoleLevelNeverAdmitsAGuest(t *testing.T) {
	for _, level := range RoleLevelValues {
		if MeetsRoleLevel(PersonTypeGuest, level) {
			t.Errorf("guest cleared %q", level)
		}
	}
}

// An unknown PERSON TYPE ranks below everything: a role this build does not
// know lets nobody in.
func TestMeetsRoleLevelRefusesUnknownPersonType(t *testing.T) {
	for _, level := range RoleLevelValues {
		if MeetsRoleLevel("moderator", level) {
			t.Errorf("unknown person type cleared %q", level)
		}
	}
}

// An unknown LEVEL ranks above everything, so a mistyped setting — or one
// that never arrived — refuses rather than opens. The opposite direction to
// the case above, which is the whole reason there are two scales.
func TestMeetsRoleLevelRefusesUnknownLevel(t *testing.T) {
	for _, personType := range []string{
		PersonTypeRegistered, PersonTypeAdmin, PersonTypeSuperAdmin,
	} {
		// `members` was accepted briefly and is gone. It must not quietly
		// behave like `registered` again.
		for _, level := range []string{"", "members", "everyone", "moderators"} {
			if MeetsRoleLevel(personType, level) {
				t.Errorf("%q cleared unknown level %q", personType, level)
			}
		}
	}
}

func TestNormalizeAndRecognizeRoleLevel(t *testing.T) {
	if got := NormalizeRoleLevel("  Administrators \n"); got != RoleLevelAdministrators {
		t.Errorf("NormalizeRoleLevel() = %q, want %q", got, RoleLevelAdministrators)
	}

	// Case and whitespace are forgiven, which means a forgiving value still
	// clears the level rather than tripping the unknown-level guard.
	if !MeetsRoleLevel(PersonTypeAdmin, " ADMINISTRATORS ") {
		t.Error("a level differing only in case and space should still rank")
	}

	if IsRoleLevel("members") {
		t.Error("`members` is not a level; it names no person type")
	}

	if !IsRoleLevel(RoleLevelSuperAdministrators) {
		t.Error("super-administrators should be recognised")
	}
}
