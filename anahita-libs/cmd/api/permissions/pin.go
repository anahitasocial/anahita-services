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
// profile and a group's administrators in the group, and for site
// administrators, who may change any post. Not for the author of a post on
// somebody else's profile or in a group they do not run, who may edit and
// remove their post and not decide what that profile shows first.
//
// Feeds and lists offer it only to whoever runs the profile (RunsProfile);
// a site administrator pins from the post's own page.
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

	if viewer.IsAdmin() {
		return true
	}

	if viewer.ID == post.GetOwnerID() {
		return true
	}

	return relationOrEmpty(rel).HasAdmin
}

// RunsProfile reports whether the viewer is the profile itself or one of
// its administrators, as against a site administrator. What changes a
// profile (pinning to it, blocking from it) is offered in feeds and lists to
// them alone, and to site administrators on the post's own page.
//
// rel is the profile's relationship to the viewer.
func RunsProfile(viewer *entities.Viewer, profileID int64, rel *entities.ActorRelation) bool {
	if viewer == nil || viewer.ID <= 0 || profileID <= 0 {
		return false
	}
	return viewer.ID == profileID || relationOrEmpty(rel).HasAdmin
}
