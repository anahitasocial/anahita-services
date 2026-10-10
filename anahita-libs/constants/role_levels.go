package constants

import "strings"

// A minimum role, as an operator writes it in a setting.
//
// Each value is the PLURAL of a person type, which is what makes a role
// rankable against a setting. There are four person types — guest,
// registered, administrator, super-administrator — and no level here names
// anything else. A level that is not the plural of a person type would be a
// person type Anahita does not have.
//
// Three settings use this scale today: INVITES_FROM, read by auth-service,
// and GROUPS_FROM and EVENTS_FROM, read by actor-service and republished by
// auth-service in NodeInfo. They share the scale so they cannot drift apart.
const (
	// RoleLevelRegistered — any registered person, and everyone above.
	RoleLevelRegistered = "registered"

	// RoleLevelAdministrators — administrators and super administrators.
	RoleLevelAdministrators = "administrators"

	// RoleLevelSuperAdministrators — super administrators only.
	RoleLevelSuperAdministrators = "super-administrators"
)

// RoleLevelValues is every value an operator may set, for validation and for
// naming the alternatives in the error when they set something else.
var RoleLevelValues = []string{
	RoleLevelRegistered,
	RoleLevelAdministrators,
	RoleLevelSuperAdministrators,
}

// IsRoleLevel reports whether a value is one this build understands.
func IsRoleLevel(level string) bool {
	for _, value := range RoleLevelValues {
		if value == level {
			return true
		}
	}

	return false
}

// NormalizeRoleLevel trims and lowercases, and does nothing else.
//
// In particular it maps no aliases. There was one, `members` for `registered`;
// it is gone. An unrecognised value is left alone so that config validation
// can refuse to start and name the three that exist, rather than being quietly
// coerced into one of them.
func NormalizeRoleLevel(level string) string {
	return strings.ToLower(strings.TrimSpace(level))
}

// MeetsRoleLevel reports whether somebody of this person type meets the
// configured minimum.
//
// The comparison is by RANK, not by equality, and never with an administrator
// shortcut above it. A permission that opens with an unconditional
// `if viewer.IsAdmin() { return true }` bypasses the setting entirely, which
// would make `super-administrators` a level that does nothing.
func MeetsRoleLevel(personType string, level string) bool {
	return personTypeRank(personType) >= roleLevelRank(level)
}

// Two scales, not one, and they fail in OPPOSITE directions.
//
// An unrecognised person type must rank BELOW everything, so nobody is let in
// by a role this build does not know. An unrecognised level must rank ABOVE
// everything, so a mistyped setting — or one that never arrived — refuses
// rather than opens.
//
// A single ranking function cannot do both: written with one, `default` sent
// guests to the top of the scale and every guest passed under `registered`.

// personTypeRank ranks a viewer's role. Guests and anything unknown are 0,
// below every configured level.
func personTypeRank(personType string) int {
	switch personType {
	case PersonTypeRegistered:
		return 1
	case PersonTypeAdmin:
		return 2
	case PersonTypeSuperAdmin:
		return 3
	default:
		return 0
	}
}

// unknownRoleLevel sits above every person type, so nothing clears a level
// this build does not recognise.
const unknownRoleLevel = 99

func roleLevelRank(level string) int {
	switch NormalizeRoleLevel(level) {
	case RoleLevelRegistered:
		return 1
	case RoleLevelAdministrators:
		return 2
	case RoleLevelSuperAdministrators:
		return 3
	default:
		return unknownRoleLevel
	}
}
