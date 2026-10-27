package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// TagDecision is what tagging target in a post by author comes to (roadmap
// row 103): whether it may be at all, and whether it waits for target's
// approval. Only a person here may be tagged: somebody on another server
// cannot approve it here, and approval comes first. Anybody may tag
// themselves, at once. A person who blocks the author may not be tagged by
// them. Otherwise the person's choice (constants.TagPermission): at once,
// each approved first (the default), or nobody. rel is target's relation to
// the author, as SocialgraphGRPC reads it with target as actor A.
func TagDecision(target *entities.AnahitaNode, author *entities.Viewer, rel *entities.ActorRelation) (allowed, pending bool) {
	if target == nil || author == nil || author.ID <= 0 {
		return false, false
	}
	if target.Type != constants.NodeActorPersonEntityType {
		return false, false
	}
	if target.ID == author.ID {
		return true, false
	}
	if rel != nil && rel.IsBlocking {
		return false, false
	}

	switch TagAccess(target) {
	case constants.TagAccessAnybody:
		return true, false
	case constants.TagAccessNobody:
		return false, false
	default:
		return true, true
	}
}

// TagAccess is a person's choice of who may tag them, the default when they
// never made one.
func TagAccess(person *entities.AnahitaNode) string {
	access := person.AddPermissionAccess(constants.TagPermissionService, constants.TagPermission)
	switch access {
	case constants.TagAccessAnybody, constants.TagAccessApprove, constants.TagAccessNobody:
		return access
	default:
		return constants.TagAccessDefault
	}
}
