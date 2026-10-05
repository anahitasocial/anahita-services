package permissions

import (
	"slices"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"
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
//
// One rule is the installation's and not the profile's: with
// MIN_CONTENT_ACCESS=registered, "public" is taken out everywhere. See package
// policy.

// ErrAudienceNotAllowed is the error code sent with a 422 when the audience
// asked for is not one of MediumAudiences.
const ErrAudienceNotAllowed = "access_not_allowed"

// MediumAudiences returns the access levels the viewer may give this post,
// widest first.
//
// ctx.Medium needs its owner attached and its author set; ctx.OwnerViewerRel
// may be nil, which reads as "no relation".
func MediumAudiences(ctx *MediumAccessContext) []string {
	return audiences(ctx.Viewer, ctx.Medium.GetOwner(), ctx.Medium.GetAuthorID(), ctx.OwnerViewerRel)
}

// PostAudiences returns the access levels a NEW post by the viewer may be
// given on owner's profile, widest first. It is MediumAudiences asked before
// the post exists, and is what a profile's response carries so the composer
// offers only audiences that will be accepted.
//
// Empty for somebody who is not signed in: they cannot post at all.
func PostAudiences(viewer *entities.Viewer, owner *entities.AnahitaNode, rel *entities.ActorRelation) []string {
	if viewer == nil || viewer.ID == 0 {
		return []string{}
	}

	return audiences(viewer, owner, viewer.ID, rel)
}

func audiences(viewer *entities.Viewer, owner *entities.AnahitaNode, authorID int64, rel *entities.ActorRelation) []string {
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
		if authorID > 0 && authorID == owner.GetID() {
			audiences = append(audiences, constants.AnahitaAccessMyself)
		}

		return policy.Allowed(audiences)
	}

	audiences := []string{
		constants.AnahitaAccessPublic,
		constants.AnahitaAccessRegistered,
		constants.AnahitaAccessFollowers,
	}

	// Administrators only: for somebody who is one, of the group or of the
	// site.
	administers := rel != nil && rel.HasAdmin
	if administers || viewer.IsAdmin() {
		audiences = append(audiences, constants.AnahitaAccessAdmins)
	}

	// Less "public", on an installation that allows no new public content.
	return policy.Allowed(audiences)
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
