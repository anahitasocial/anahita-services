package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// Events: an actor people say they are going to.

// CanRsvp reports whether the viewer may answer an event, going or maybe.
//
// Somebody who can see it and whom it has not blocked, as for following
// anything. And somebody invited, whether or not they could otherwise see
// it: the invitation is the event's consent. An event that lets in the
// followers of the group hosting it has invited all of them at once.
//
// Whether there is a place left, and whether the event is still to come,
// are answered by the graph when the answer is recorded.
func CanRsvp(viewer *entities.Viewer, event *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 || event == nil {
		return false
	}

	if !event.Enabled || event.DeletedAt != nil || event.ArchivedAt != nil {
		return false
	}

	rel = relationOrEmpty(rel)
	if rel.IsBlocking {
		return false
	}

	// Invited, one by one or as a follower of the group hosting it.
	if rel.HasInvite || rel.ViaHost {
		return true
	}

	ok, _ := NewActorPermissions().CanRead(&ActorAccessContext{
		Viewer:         viewer,
		Actor:          event,
		ActorViewerRel: rel,
	})

	return ok
}

// CanSeeOnlineURL reports whether the viewer is shown where to join an
// event online: whoever is going, and whoever looks after the event.
//
// Not somebody who said maybe, was invited, or merely can see the event. An
// address to a meeting is given to the people coming to it.
func CanSeeOnlineURL(viewer *entities.Viewer, event *entities.AnahitaNode, rel *entities.ActorRelation, details *entities.EventDetails) bool {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 || event == nil || details == nil {
		return false
	}

	if viewer.IsAdmin() || relationOrEmpty(rel).HasAdmin {
		return true
	}

	return details.ViewerRsvp == constants.EventRsvpGoing
}

// CanCancelEvent reports whether the viewer may call an event off: whoever
// looks after it.
func CanCancelEvent(viewer *entities.Viewer, event *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 || event == nil {
		return false
	}

	return viewer.IsAdmin() || relationOrEmpty(rel).HasAdmin
}
