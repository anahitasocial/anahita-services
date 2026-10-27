package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// Following, and asking to follow.
//
// Who may make a follow, take one away, ask for one, and answer the asking.
// Plain rules over what the caller has already read, so they can be tested
// without a graph.

// FollowContext is one follow being asked about: the source would follow
// the target, and the viewer is whoever is asking for it to happen.
type FollowContext struct {
	Viewer *entities.Viewer
	Source *entities.AnahitaNode
	Target *entities.AnahitaNode
	// The target's relation to the viewer, and to the source. They are the
	// same when the viewer is the source.
	TargetViewerRel *entities.ActorRelation
	TargetSourceRel *entities.ActorRelation
}

func (ctx *FollowContext) usable() bool {
	return ctx != nil &&
		ctx.Viewer != nil && ctx.Viewer.Authenticated && ctx.Viewer.ID > 0 &&
		ctx.Source != nil && ctx.Target != nil &&
		ctx.Source.ID > 0 && ctx.Target.ID > 0 &&
		ctx.Source.ID != ctx.Target.ID
}

func (ctx *FollowContext) viewerIsSource() bool {
	return ctx.Source.ID == ctx.Viewer.ID
}

// looksAfterTarget reports whether the viewer answers for the target: it is
// their own profile, they administer it, or they administer the site.
func (ctx *FollowContext) looksAfterTarget() bool {
	return ctx.Viewer.IsAdmin() ||
		ctx.Viewer.ID == ctx.Target.ID ||
		relationOrEmpty(ctx.TargetViewerRel).HasAdmin
}

// CanFollow reports whether the viewer may make the source a follower of the
// target.
//
// Following is something a person does for themselves: the source has to be
// the viewer, and the viewer has to be able to see what they are following.
// It used to be enough to see a PERSON for anybody at all to be made their
// follower, so one request could make a stranger follow another stranger.
//
// The one case of adding somebody else is a group's "follower" permission
// (CanAddOthersAsFollowers). Nobody adds followers to a person.
func CanFollow(ctx *FollowContext) bool {
	if !ctx.usable() {
		return false
	}

	if relationOrEmpty(ctx.TargetSourceRel).IsBlocking {
		return false
	}

	if ok, _ := NewActorPermissions().CanRead(&ActorAccessContext{
		Viewer:         ctx.Viewer,
		Actor:          ctx.Target,
		ActorViewerRel: relationOrEmpty(ctx.TargetViewerRel),
	}); !ok {
		return false
	}

	if ctx.viewerIsSource() {
		return true
	}

	return CanAddOthersAsFollowers(ctx.Viewer, ctx.Target, ctx.TargetViewerRel)
}

// CanRemoveFollower reports whether the viewer may end the source's follow
// of the target: by unfollowing, which is always theirs to do, or as
// whoever looks after the target, who may remove a follower.
//
// It used to be anybody who could see the target, so any follow of a public
// profile could be ended by anyone signed in.
func CanRemoveFollower(ctx *FollowContext) bool {
	if !ctx.usable() {
		return false
	}

	return ctx.viewerIsSource() || ctx.looksAfterTarget()
}

// CanRequestToFollow reports whether the viewer may ask to follow the
// target.
//
// Asking is for a profile that has said it may be asked. It does not need
// the asker to be able to see the profile: a profile that can be seen can be
// followed outright, so the ones worth asking are the ones that cannot.
// Nobody asks on somebody else's behalf.
func CanRequestToFollow(ctx *FollowContext) bool {
	if !ctx.usable() || !ctx.viewerIsSource() {
		return false
	}

	// A website is public, and followed outright: there is nothing to ask.
	if !ctx.Target.AllowFollowRequest || ctx.Target.Type == constants.NodeActorWebsiteEntityType {
		return false
	}

	rel := relationOrEmpty(ctx.TargetSourceRel)

	// IsLeading: the source already follows the target.
	return !rel.IsBlocking && !rel.IsLeading
}

// CanAcceptFollowRequest reports whether the viewer may turn a request to
// follow the target into a follow: whoever looks after the target.
func CanAcceptFollowRequest(ctx *FollowContext) bool {
	return ctx.usable() && ctx.looksAfterTarget()
}

// CanDropFollowRequest reports whether the viewer may remove a request to
// follow the target: whoever looks after the target, ignoring it, or
// whoever sent it, taking it back.
func CanDropFollowRequest(ctx *FollowContext) bool {
	return ctx.usable() && (ctx.viewerIsSource() || ctx.looksAfterTarget())
}
