package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// CanMention says whether a post by author may name target: whether a
// mention of it is kept, as an edge, and its person told of it. A group or
// an event may always be named, and anybody may name themselves. A person
// says who may in their permissions (constants.MentionPermission: anybody
// signed in, unless they chose followers, the people they follow, mutuals
// or nobody), and nobody they block may. rel is target's relation to the
// author, as SocialgraphGRPC reads it with target as actor A.
func CanMention(target *entities.AnahitaNode, author *entities.Viewer, rel *entities.ActorRelation) bool {
	if target == nil || author == nil {
		return false
	}
	if target.Type != constants.NodeActorPersonEntityType || target.ID == author.ID {
		return true
	}
	if rel == nil {
		rel = &entities.ActorRelation{}
	}
	if rel.IsBlocking {
		return false
	}

	access := target.AddPermissionAccess(constants.MentionPermissionService, constants.MentionPermission)
	if access == "" {
		access = constants.AnahitaAccessRegistered
	}
	// The author is somebody here, signed in, writing.
	writer := entities.NewViewer()
	writer.ID = author.ID
	writer.PersonType = author.PersonType
	writer.Authenticated = true
	return AddAccessAllows(access, &entities.ActorRelation{
		IsLeading:   rel.IsLeading,
		IsFollowing: rel.IsFollowing,
		IsMutual:    rel.IsMutual,
	}, writer)
}
