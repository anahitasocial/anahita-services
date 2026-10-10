package permissions

import "anahita-libs/data/entities"

// Invitations: asking somebody to follow a group, which they then accept or
// decline. Nobody is made a follower of anything without saying yes.

// CanInvite reports whether the viewer may invite people to follow an
// actor: the actor's "invite" permission, measured against the viewer.
//
// The setting is `admins` or `followers`, and `admins` where an actor has
// never said. Site administrators always may. Only for an actor the viewer
// can see and that is still in use. Nobody is invited to follow a person:
// people are followed, or asked.
//
// Who can be invited is a separate question, answered for each person by
// the graph: the inviter's own followers.
func CanInvite(viewer *entities.Viewer, actor *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 || actor == nil || actor.IsPerson() {
		return false
	}

	if !actor.Enabled || actor.DeletedAt != nil || actor.ArchivedAt != nil {
		return false
	}

	if viewer.IsAdmin() {
		return true
	}

	rel = relationOrEmpty(rel)

	if ok, _ := NewActorPermissions().CanRead(&ActorAccessContext{
		Viewer:         viewer,
		Actor:          actor,
		ActorViewerRel: rel,
	}); !ok {
		return false
	}

	return AddAccessAllows(actor.AddPermissionAccess("socialgraph-service", "invite"), rel, viewer)
}

// CanSeeInvites reports whether the viewer may see who has been invited to
// an actor and is yet to answer: whoever looks after it.
func CanSeeInvites(viewer *entities.Viewer, actor *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 || actor == nil {
		return false
	}

	return viewer.IsAdmin() || relationOrEmpty(rel).HasAdmin
}

// CanWithdrawInvite reports whether the viewer may take an invitation back:
// whoever made it, or whoever looks after the actor.
func CanWithdrawInvite(viewer *entities.Viewer, actor *entities.AnahitaNode, rel *entities.ActorRelation, inviterID int64) bool {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 || actor == nil {
		return false
	}

	return viewer.ID == inviterID || CanSeeInvites(viewer, actor, rel)
}
