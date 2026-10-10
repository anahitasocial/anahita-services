package handlers

import (
	"anahita-libs/cmd/api/permissions"
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/responses"
	"log"
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

	// Somebody invited to an event is told when it is. That is what an
	// invitation is for. Somebody who may only ask to follow it is not.
	if limited.IsInvited && actor.Type == an_constants.NodeActorEventEntityType {
		limited.Event = limitedEvent(env, actor, ctx.Viewer)
	}

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

// limitedEvent is when an event is and how full, for somebody invited to it
// who may not otherwise see it. Where to join it online is never in it.
func limitedEvent(env *Env, actor *entities.AnahitaNode, viewer *entities.Viewer) *responses.Event {
	details, err := env.Srv.EventsGRPC.Read([]int64{actor.ID}, viewer.ID)
	if err != nil {
		log.Println("limited event:", err)
		return nil
	}

	shown := *actor
	shown.Event = details[actor.ID]
	shown.EventShowsOnlineURL = false

	return responses.NewEvent(&shown)
}
