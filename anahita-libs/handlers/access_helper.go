package handlers

import "anahita-libs/constants"

// DetermineMediumNodeAccess determines the access level for a medium node
func DetermineMediumNodeAccess(accessLevel string, ownerAccess string) string {
	switch accessLevel {
	case "public":
		return constants.AnahitaAccessPublic
	case "registered":
		return constants.AnahitaAccessRegistered
	case "followers":
		return constants.AnahitaAccessFollowers
	case "leaders":
		return constants.AnahitaAccessLeaders
	case "mutuals":
		return constants.AnahitaAccessMutuals
	case "admins":
		return constants.AnahitaAccessAdmins
	case "myself":
		return constants.AnahitaAccessMyself
	default:
		return ownerAccess
	}
}

// DetermineActorNodeAccess determines the access level for an actor node
func DetermineActorNodeAccess(accessLevel string) string {
	switch accessLevel {
	case "public":
		return constants.AnahitaAccessPublic
	case "registered":
		return constants.AnahitaAccessRegistered
	case "followers":
		return constants.AnahitaAccessFollowers
	case "admins":
		return constants.AnahitaAccessAdmins
	default:
		// Fail CLOSED. An unrecognised value used to become "public", so a
		// typo, a renamed constant or a caller that skipped validation would
		// silently publish an actor rather than fail.
		//
		// Unreachable today — both callers validate `oneof` first and reject
		// anything else with a 400 — which is exactly why it is worth setting
		// correctly now: the next caller may not, and the wrong default there
		// is invisible.
		return constants.AnahitaAccessAdmins
	}
}

// DeterminePersonNodeAccess determines the access level for a person node
func DeterminePersonNodeAccess(accessLevel string) string {
	switch accessLevel {
	case "public":
		return constants.AnahitaAccessPublic
	case "registered":
		return constants.AnahitaAccessRegistered
	case "followers":
		return constants.AnahitaAccessFollowers
	case "leaders":
		return constants.AnahitaAccessLeaders
	case "mutuals":
		return constants.AnahitaAccessMutuals
	case "admins":
		return constants.AnahitaAccessAdmins
	case "myself":
		return constants.AnahitaAccessMyself
	default:
		// Fail closed, for the reason above. "myself" is the most restrictive
		// a person can be — the draft state — so an unrecognised value hides
		// the profile rather than publishing it.
		return constants.AnahitaAccessMyself
	}
}

// DetermineActorNodeWhoCanAddFollowers determines who can add followers to an actor node
func DetermineActorNodeWhoCanAddFollowers(whoCanAddFollowers string) string {
	switch whoCanAddFollowers {
	case "admins":
		return constants.AnahitaAccessAdmins
	case "followers":
		return constants.AnahitaAccessFollowers
	default:
		return constants.AnahitaAccessAdmins // Default to admins if not specified
	}
}
