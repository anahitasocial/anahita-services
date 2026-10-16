package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// CanBlockFromOwner reports whether the viewer may block, from a group or
// an event, whoever wrote something in it: the group blocking them, not the
// viewer.
//
// It is for the group's administrators, who look after what is in it. A
// person's own block does not reach the groups they administer (decided
// 2026-10-16); the group blocks in its own right. Not offered to site
// administrators here, who have their own ways of acting against somebody.
//
// `owner` is the group or event the post is on, or the one at the top of a
// reply's thread; `authorID` is who wrote it. rel is the owner's
// relationship to the viewer.
func CanBlockFromOwner(viewer *entities.Viewer, owner *entities.AnahitaNode, authorID int64, rel *entities.ActorRelation) bool {
	if viewer == nil || owner == nil || viewer.ID <= 0 || !viewer.Authenticated {
		return false
	}
	if !viewer.HasScope(constants.OAuthScopeSocialgraphWrite) {
		return false
	}
	// A person's profile is theirs, and they block as themselves.
	if owner.GetID() <= 0 || !owner.IsActor() || owner.IsPerson() {
		return false
	}
	if authorID <= 0 || authorID == viewer.ID || authorID == owner.GetID() {
		return false
	}
	return relationOrEmpty(rel).HasAdmin
}
