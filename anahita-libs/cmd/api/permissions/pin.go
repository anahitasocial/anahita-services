package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// CanPin reports whether the viewer may pin a post to the profile it is on,
// or take the pin off.
//
// A pin is the profile's, not the post's: it says what the profile leads
// with. So it is for whoever the profile belongs to, a person on their own
// profile and a group's administrators in the group. Not for the author of a
// post on somebody else's profile or in a group they do not run, who may edit
// and remove their post and not decide what that profile shows first. Nor
// for site administrators: what they may do to somebody's posts is
// moderation, and choosing what a profile leads with is not.
//
// Only a post at the top level. A reply is read in its thread.
//
// rel is the relationship of the post's owner to the viewer.
func CanPin(viewer *entities.Viewer, post *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	if viewer == nil || post == nil || viewer.ID <= 0 || !viewer.Authenticated {
		return false
	}

	if !viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false
	}

	if post.GetID() <= 0 || !post.IsMedium() || post.RootID > 0 || post.DeletedAt != nil {
		return false
	}

	if viewer.ID == post.GetOwnerID() {
		return true
	}

	return relationOrEmpty(rel).HasAdmin
}
