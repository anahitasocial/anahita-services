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

	limited.ViaHost = ctx.ActorViewerRel != nil && ctx.ActorViewerRel.ViaHost

	// Somebody invited to an event is told what it is and when. That is
	// what an invitation is for, whether it came to them alone or to
	// everybody who follows the group hosting it. Somebody who may only
	// ask to follow it is not.
	if (limited.IsInvited || limited.ViaHost) && actor.Type == an_constants.NodeActorEventEntityType {
		limited.Event = limitedEvent(env, actor, ctx.Viewer)
		limited.Body = actor.Body
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

// eventListItem is an event as it goes into a list: whole for somebody who
// may see it, and as its name, picture and time for somebody who was
// invited to it, alone or as a follower of the group hosting it. Nil for
// somebody who is neither, to whom it is not shown.
func eventListItem(event *entities.AnahitaNode, rel *entities.ActorRelation, viewer *entities.Viewer) any {
	if rel == nil {
		rel = &entities.ActorRelation{}
	}

	canRead, _ := permissions.NewActorPermissions().CanRead(&permissions.ActorAccessContext{
		Viewer:         viewer,
		Actor:          event,
		ActorViewerRel: rel,
	})
	if canRead {
		event.IsLeadingViewer = rel.IsLeading
		event.IsAdminedByViewer = rel.HasAdmin
		event.IsInvitedViewer = rel.HasInvite

		return (&responses.ActorResponse{}).MapNodeItem(event)
	}

	if !permissions.CanSeeLimitedProfile(viewer, event, rel) {
		return nil
	}

	limited := responses.NewLimitedActor(event, rel.HasRequested)
	limited.IsInvited = rel.HasInvite
	limited.ViaHost = rel.ViaHost
	if rel.HasInvite || rel.ViaHost {
		// Never the way in: that is for whoever is going.
		event.EventShowsOnlineURL = false
		limited.Event = responses.NewEvent(event)
		limited.Body = event.Body
	}

	return limited
}
