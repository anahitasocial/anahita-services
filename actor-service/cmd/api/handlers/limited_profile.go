package handlers

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/responses"
)

// limitedProfile is what the viewer is sent of a profile they may not see:
// its name and picture, when it lets people ask to follow it. Nil when it
// does not, and the profile is then refused whole as before.
func limitedProfile(env *Env, actor *entities.AnahitaNode, ctx *permissions.ActorAccessContext) *responses.LimitedActor {
	if !permissions.CanSeeLimitedProfile(ctx.Viewer, actor, ctx.ActorViewerRel) {
		return nil
	}

	batchPresignPaths(env, []*entities.AnahitaNode{actor})

	limited := responses.NewLimitedActor(actor, ctx.ActorViewerRel != nil && ctx.ActorViewerRel.HasRequested)
	limited.IsInvited = ctx.ActorViewerRel != nil && ctx.ActorViewerRel.HasInvite

	return limited
}

// listsAskable reports whether a list of people or of groups also shows the
// ones the viewer may only ask to follow.
//
// For somebody signed in looking at the ordinary list. Not the featured
// list, which is what a new member is offered to follow outright, and not an
// administrator's list of disabled accounts.
func listsAskable(viewer *entities.Viewer, featured bool, disabled bool) bool {
	return viewer != nil && viewer.Authenticated && viewer.ID > 0 && !featured && !disabled
}
