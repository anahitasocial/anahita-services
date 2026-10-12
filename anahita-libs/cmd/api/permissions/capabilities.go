package permissions

import (
	"fmt"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// What a viewer may add to a profile or a post, as the answers the server
// sends with a response so the web app can show only the controls that will
// work.
//
// Every answer here is the check the server enforces, not a second copy of it.
// The web app used to derive them itself and drifted: it read any
// add_permission in a service as if it were the one for the composer, knew
// only followers and admins, and showed the comment box and like button to
// everybody signed in.

// CanPostTo reports whether the viewer may create an entity of the given kind
// under service on owner's profile. MediumPermissions.CanAdd is this plus the
// media:write scope.
//
// owner has to come from the actor read, which merges the actor_features
// defaults in; rel is owner's relationship to the viewer.
func CanPostTo(viewer *entities.Viewer, owner *entities.AnahitaNode, rel *entities.ActorRelation, service, entity string) (bool, error) {
	// Nobody posts on the profile of somebody on another server, an
	// administrator included: it is not this installation's profile.
	if owner != nil && owner.IsRemote() {
		return false, fmt.Errorf("actor %d is on another server", owner.GetID())
	}
	if viewer.IsAdmin() {
		return true, nil
	}

	if owner == nil || owner.GetID() == 0 {
		return false, fmt.Errorf("no owner to add %s to", entity)
	}

	// The owner can always add to their own profile.
	if viewer.ID > 0 && viewer.ID == owner.GetID() {
		return true, nil
	}

	rel = relationOrEmpty(rel)

	ok, err := NewActorPermissions().CanRead(&ActorAccessContext{
		Viewer:         viewer,
		Actor:          owner,
		ActorViewerRel: rel,
	})
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("viewer %d is not allowed to read actor %d", viewer.ID, owner.GetID())
	}

	// An entity the owner has no permission for answers "" and admits only
	// the owner's administrators.
	if AddAccessAllows(owner.AddPermissionAccess(service, entity), rel, viewer) {
		return true, nil
	}

	return false, fmt.Errorf("viewer %d does not have permission to add %s to actor %d", viewer.ID, entity, owner.GetID())
}

// Composers lists the composers on owner's profile the viewer may use: every
// enabled feature's composers that CanPostTo allows. A composer is named for
// the entity it creates.
func Composers(viewer *entities.Viewer, owner *entities.AnahitaNode, rel *entities.ActorRelation) []string {
	composers := []string{}

	if viewer == nil || !viewer.HasScope(constants.OAuthScopeMediaWrite) || owner == nil {
		return composers
	}

	features, err := owner.GetFeatures()
	if err != nil {
		return composers
	}

	for _, feature := range features {
		if !feature.Enabled {
			continue
		}
		for _, composer := range feature.Composers {
			if ok, _ := CanPostTo(viewer, owner, rel, feature.Service, composer); ok {
				composers = append(composers, composer)
			}
		}
	}

	return composers
}

// CanReplyToPost reports whether the viewer may reply to post: CanReply,
// asked with what a list of posts has to hand.
//
// rel is the relationship of the post's owner to the viewer. A list does not
// read the author's as well, so for a post that somebody wrote on a profile
// other than their own, and limited to people they follow or who follow
// them, this says no where the thread may say yes. The thread's own answer
// is the full one, and the server decides a reply by that.
//
// post.Owner has to be loaded.
func CanReplyToPost(viewer *entities.Viewer, post *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	if viewer == nil || post == nil {
		return false
	}

	ctx := &ReplyAccessContext{
		Viewer:             viewer,
		Root:               post,
		RootOwnerViewerRel: rel,
	}

	// On its author's own profile the two relationships are one.
	if post.GetAuthorID() == post.GetOwnerID() {
		ctx.RootAuthorViewerRel = rel
	}

	return CanReply(ctx)
}

// CanLike reports whether the viewer may like post, or a reply under it:
// be able to read the post, then meet the owner's like permission for the
// post's service. The owner and site administrators always may.
//
// like-service enforces exactly this. post.Owner has to be loaded.
func CanLike(viewer *entities.Viewer, post *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	if viewer == nil || !viewer.Authenticated || !viewer.HasScope(constants.OAuthScopeLikesWrite) {
		return false
	}
	// A post from another server is not liked from here yet: its author
	// would never be told, so the like would be a lie to whoever made it.
	if post != nil && post.IsRemote() {
		return false
	}

	if viewer.IsAdmin() {
		return true
	}

	if post == nil || post.GetID() == 0 {
		return false
	}
	owner := post.GetOwner()
	if owner == nil || owner.GetID() == 0 {
		return false
	}

	rel = relationOrEmpty(rel)

	if ok, _ := NewMediumPermissions().CanRead(&MediumAccessContext{
		Viewer:         viewer,
		Medium:         post,
		OwnerViewerRel: rel,
	}); !ok {
		return false
	}

	if owner.GetID() == viewer.ID {
		return true
	}

	return AddAccessAllows(owner.AddPermissionAccess(post.GetService(), "like"), rel, viewer)
}

// CanAddOthersAsFollowers reports whether the viewer may make somebody else
// a follower of a group, without that person saying yes: site
// administrators alone, for moderation and for moving data.
//
// It was the group's "follower" permission, which let a group's
// administrators, or its followers, put anybody in it. People are now
// invited (CanInvite) and choose. The stored setting is left where it is
// and no longer read.
//
// Joining a group yourself is not this question. Nobody adds followers to a
// person.
func CanAddOthersAsFollowers(viewer *entities.Viewer, group *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 || group == nil || group.IsPerson() {
		return false
	}

	return viewer.IsAdmin()
}
