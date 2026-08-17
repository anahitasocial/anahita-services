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
		return constants.AnahitaAccessPublic
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
		return constants.AnahitaAccessPublic
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
