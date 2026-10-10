package permissions

import "anahita-libs/data/entities"

// CanSeeLimitedProfile reports whether somebody who may NOT see a profile is
// shown that it is there: its name and picture, and a way to ask to follow
// it.
//
// A profile that lets people ask (allow_follow_request), or one the viewer
// has been invited to follow, which has to be seen to be answered. Only to
// somebody signed in whom it has not blocked. Without this nobody could ask:
// the profile was refused whole, so there was nothing to ask from. A profile
// that does not let people ask stays as it was, refused whole.
//
// It is asked after CanRead has said no. It grants nothing else: no posts,
// no followers, no description.
func CanSeeLimitedProfile(viewer *entities.Viewer, actor *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 || actor == nil {
		return false
	}

	if !actor.Enabled || actor.DeletedAt != nil || actor.ArchivedAt != nil {
		return false
	}

	rel = relationOrEmpty(rel)
	if rel.IsBlocking {
		return false
	}

	return actor.AllowFollowRequest || rel.HasInvite
}
