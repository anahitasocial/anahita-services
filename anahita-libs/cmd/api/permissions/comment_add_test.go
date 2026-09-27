package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

const (
	ownerID  = 1
	authorID = 2
)

// commentOn builds a new comment under a post on owner's profile, the way the
// comment handler hands it to CanAdd once it has loaded the post and owner.
func commentOn(owner *entities.AnahitaNode, postAccess string) *entities.AnahitaNode {
	post := entities.NewAnahitaNode()
	post.ID = 10
	post.Type = "node.medium.text-service.note.v1"
	post.Service = "text-service"
	post.Access = postAccess
	post.OwnerID = owner.ID
	post.Owner = owner
	post.CreatedBy = authorID

	comment := entities.NewAnahitaNode()
	comment.ParentID = post.ID
	comment.Parent = post
	return comment
}

func canComment(viewerID int64, comment *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	v := viewerWith(constants.PersonTypeRegistered, constants.OAuthScopeCommentsWrite)
	v.ID = viewerID
	ok, _ := NewCommentPermissions().CanAdd(&CommentAccessContext{
		Viewer:               v,
		Comment:              comment,
		ParentOwnerViewerRel: rel,
	})
	return ok
}

func TestCommentCanAdd(t *testing.T) {
	stranger := &entities.ActorRelation{}
	follower := &entities.ActorRelation{IsLeading: true}
	leader := &entities.ActorRelation{IsFollowing: true}
	admin := &entities.ActorRelation{HasAdmin: true}

	publicDefault := ownerWith(ownerID, "public")
	publicFollowers := ownerWith(ownerID, "public", perm("comment", "followers"))
	followersProfile := ownerWith(ownerID, "followers", perm("comment", "registered"))

	cases := []struct {
		name    string
		viewer  int64
		comment *entities.AnahitaNode
		rel     *entities.ActorRelation
		want    bool
	}{
		{"public default: a stranger", 100, commentOn(publicDefault, "public"), stranger, true},
		{"public followers: a stranger", 100, commentOn(publicFollowers, "public"), stranger, false},
		{"public followers: a follower", 100, commentOn(publicFollowers, "public"), follower, true},
		{"followers profile: a follower", 100, commentOn(followersProfile, "public"), follower, true},
		// Can read the profile — it follows them — but is not a follower.
		{"followers profile: a leader", 100, commentOn(followersProfile, "public"), leader, false},
		{"followers profile: a stranger", 100, commentOn(followersProfile, "public"), stranger, false},

		// Always allowed.
		{"the owner", ownerID, commentOn(publicFollowers, "public"), stranger, true},
		{"the post's author", authorID, commentOn(publicFollowers, "public"), stranger, true},
		{"an admin of the profile", 100, commentOn(publicFollowers, "admins"), admin, true},

		// Reading comes first: the comment setting never widens it.
		{"a post the viewer cannot read", 100, commentOn(publicDefault, "admins"), stranger, false},
		{"no relationship at all", 100, commentOn(publicDefault, "public"), nil, true},
	}

	for _, c := range cases {
		if got := canComment(c.viewer, c.comment, c.rel); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// The live hole: the handler used to hand CanAdd the empty placeholder parent
// NewAnahitaNode makes, and against it a stranger could comment on a post they
// could not read. Without a loaded post and owner the answer is no.
func TestCommentCanAddRefusesAnUnloadedParent(t *testing.T) {
	if canComment(100, entities.NewAnahitaNode(), &entities.ActorRelation{}) {
		t.Error("allowed a comment under a parent that was never loaded")
	}
}

const commentAuthorID = 3

// existingComment is a comment by commentAuthorID under a post on owner's
// profile, loaded the way the comment handlers now load it.
func existingComment(owner *entities.AnahitaNode, postAccess string) *entities.AnahitaNode {
	comment := commentOn(owner, postAccess)
	comment.ID = 20
	comment.CreatedBy = commentAuthorID
	return comment
}

func commentCtx(viewerID int64, personType string, comment *entities.AnahitaNode, rel *entities.ActorRelation) *CommentAccessContext {
	v := viewerWith(personType, constants.OAuthScopeCommentsWrite)
	v.ID = viewerID
	return &CommentAccessContext{Viewer: v, Comment: comment, ParentOwnerViewerRel: rel}
}

// Editing is the author's alone. An edited comment still shows its author's
// name, so anybody else editing it would put words under that name.
func TestCommentCanEdit(t *testing.T) {
	owner := ownerWith(ownerID, "public")
	reg := constants.PersonTypeRegistered

	cases := []struct {
		name string
		ctx  *CommentAccessContext
		want bool
	}{
		{"the author", commentCtx(commentAuthorID, reg, existingComment(owner, "public"), nil), true},
		{"the profile owner", commentCtx(ownerID, reg, existingComment(owner, "public"), nil), false},
		{"the post's author", commentCtx(authorID, reg, existingComment(owner, "public"), nil), false},
		{"an admin of the profile", commentCtx(100, reg, existingComment(owner, "public"), &entities.ActorRelation{HasAdmin: true}), false},
		{"a site administrator", commentCtx(100, constants.PersonTypeAdmin, existingComment(owner, "public"), nil), false},
		{"a super administrator", commentCtx(100, constants.PersonTypeSuperAdmin, existingComment(owner, "public"), nil), false},
		// Lost sight of the post — blocked, or the profile narrowed.
		{"the author, post no longer readable", commentCtx(commentAuthorID, reg, existingComment(owner, "admins"), nil), false},
	}

	for _, c := range cases {
		if got, _ := NewCommentPermissions().CanEdit(c.ctx); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// Deleting is moderation: the author, the profile's owner and administrators,
// and site administrators.
func TestCommentCanDelete(t *testing.T) {
	owner := ownerWith(ownerID, "public")
	reg := constants.PersonTypeRegistered

	cases := []struct {
		name string
		ctx  *CommentAccessContext
		want bool
	}{
		{"the author", commentCtx(commentAuthorID, reg, existingComment(owner, "public"), nil), true},
		// Taking your own words down works even once you cannot see the post.
		{"the author, post no longer readable", commentCtx(commentAuthorID, reg, existingComment(owner, "admins"), nil), true},
		{"the profile owner", commentCtx(ownerID, reg, existingComment(owner, "public"), nil), true},
		{"an admin of the profile", commentCtx(100, reg, existingComment(owner, "public"), &entities.ActorRelation{HasAdmin: true}), true},
		{"a site administrator", commentCtx(100, constants.PersonTypeAdmin, existingComment(owner, "public"), nil), true},
		{"a super administrator", commentCtx(100, constants.PersonTypeSuperAdmin, existingComment(owner, "public"), nil), true},
		// Wrote the post, but the profile is somebody else's.
		{"the post's author", commentCtx(authorID, reg, existingComment(owner, "public"), nil), false},
		{"a follower", commentCtx(100, reg, existingComment(owner, "public"), &entities.ActorRelation{IsLeading: true}), false},
		{"a guest", commentCtx(0, constants.PersonTypeGuest, existingComment(owner, "public"), nil), false},
	}

	for _, c := range cases {
		if got, _ := NewCommentPermissions().CanDelete(c.ctx); got != c.want {
			t.Errorf("delete, %s: got %v, want %v", c.name, got, c.want)
		}
		if got, _ := NewCommentPermissions().CanAdministrate(c.ctx); got != c.want {
			t.Errorf("administrate, %s: got %v, want %v", c.name, got, c.want)
		}
	}

	// Against the placeholder parent the owner is unknown, and nobody but the
	// author and site administrators is recognised.
	unloaded := entities.NewAnahitaNode()
	unloaded.CreatedBy = commentAuthorID
	if ok, _ := NewCommentPermissions().CanDelete(commentCtx(0, reg, unloaded, nil)); ok {
		t.Error("an unloaded parent let a viewer with id 0 delete as its owner")
	}
}
