package permissions

import (
	"slices"

	"anahita-libs/constants"
)

// Which audiences a post may be given, and by whom.
//
// A post's access is one of seven words, and they do not all make sense
// everywhere. Each is judged against the profile the post is ON, not against
// its author: "followers" means the followers of that profile, "admins" its
// administrators, "myself" its owner. So:
//
//   - "admins" means nothing on a person's profile, which has none, and
//     "leaders", "mutuals" and "myself" mean nothing in a group.
//   - "myself" on SOMEBODY ELSE'S profile would be a post only its author and
//     that person can see. That is a private message, which this platform
//     deliberately does not have: it was removed, with the data, in 2026-10.
//   - "admins" from a member who is not one would be the same thing addressed
//     to a group's administrators.
//
// The request structs accept all seven for every post, because a validation
// tag cannot know where the post is going. This is the check that does.
//
// It answers both the create and the change-access handlers, so a post cannot
// be given by editing an audience it could not have been created with.

// ErrAudienceNotAllowed is the error code sent with a 422 when the audience
// asked for is not one of MediumAudiences.
const ErrAudienceNotAllowed = "access_not_allowed"

// MediumAudiences returns the access levels the viewer may give this post,
// widest first.
//
// ctx.Medium needs its owner attached and its author set; ctx.OwnerViewerRel
// may be nil, which reads as "no relation".
func MediumAudiences(ctx *MediumAccessContext) []string {
	owner := ctx.Medium.GetOwner()
	if owner == nil {
		return nil
	}

	if owner.IsPerson() {
		audiences := []string{
			constants.AnahitaAccessPublic,
			constants.AnahitaAccessRegistered,
			constants.AnahitaAccessFollowers,
			constants.AnahitaAccessLeaders,
			constants.AnahitaAccessMutuals,
		}

		// Only me: only on a post the profile's owner wrote on their own
		// profile. Keyed on the author, not on who is asking, so an
		// administrator tidying somebody's post cannot make it one either.
		if ctx.Medium.GetAuthorID() > 0 && ctx.Medium.GetAuthorID() == ctx.Medium.GetOwnerID() {
			audiences = append(audiences, constants.AnahitaAccessMyself)
		}

		return audiences
	}

	audiences := []string{
		constants.AnahitaAccessPublic,
		constants.AnahitaAccessRegistered,
		constants.AnahitaAccessFollowers,
	}

	// Administrators only: for somebody who is one, of the group or of the
	// site.
	administers := ctx.OwnerViewerRel != nil && ctx.OwnerViewerRel.HasAdmin
	if administers || ctx.Viewer.IsAdmin() {
		audiences = append(audiences, constants.AnahitaAccessAdmins)
	}

	return audiences
}

// IsMediumAudienceAllowed reports whether the viewer may give this post this
// access level.
//
// An empty level is allowed: it means "not said", and the handlers then fall
// back to a default of their own.
func IsMediumAudienceAllowed(ctx *MediumAccessContext, access string) bool {
	if access == "" {
		return true
	}

	return slices.Contains(MediumAudiences(ctx), access)
}
