package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// THE RULE THIS GUARDS: what came from another server is not this
// installation's to change. Nobody posts on such a profile or changes such
// a post, a site administrator included, and nobody likes one: a like
// would never reach whoever it was meant for. It may be answered, since an
// answer is the answerer's own post and is sent to whoever it answers.
func TestWhatCameFromAnotherServerIsNotActedOn(t *testing.T) {
	admin := entities.NewViewer()
	admin.ID = 1
	admin.Authenticated = true
	admin.PersonType = "super-administrator"
	admin.Scopes = []string{constants.OAuthScopeMediaWrite, constants.OAuthScopeLikesWrite}

	alice := entities.NewAnahitaNode()
	alice.ID = 50
	alice.Type = constants.NodeActorActivityPubPersonEntityType
	alice.URI = "https://mastodon.example/users/alice"
	alice.Access = "public"
	alice.Enabled = true

	note := entities.NewAnahitaNode()
	note.ID = 60
	note.Type = constants.NodeMediumNoteEntityType
	note.URI = "https://mastodon.example/users/alice/statuses/1"
	note.Access = "public"
	note.Enabled = true
	note.OwnerID, note.CreatedBy = alice.ID, alice.ID
	note.Owner, note.Author = alice, alice
	note.CommentStatus = true

	if !admin.IsAdmin() {
		t.Fatal("the test's administrator is not one")
	}

	if ok, _ := CanPostTo(admin, alice, nil, "text-service", "note"); ok {
		t.Error("an administrator may post on the profile of somebody on another server")
	}
	if ok, _ := NewMediumPermissions().CanEdit(&MediumAccessContext{Viewer: admin, Medium: note, OwnerViewerRel: &entities.ActorRelation{}}); ok {
		t.Error("an administrator may change a post from another server")
	}
	if CanLike(admin, note, nil) {
		t.Error("a post from another server may be liked")
	}
	// It may be answered, by whoever is signed in and may write posts:
	// who its author lets reply is its own server's to say.
	reader := entities.NewViewer()
	reader.ID, reader.Authenticated = 7, true
	reader.Scopes = []string{constants.OAuthScopeMediaWrite}
	if !CanReplyToPost(reader, note, &entities.ActorRelation{}) {
		t.Error("a post from another server may not be replied to")
	}
	if CanReplyToPost(entities.NewViewer(), note, nil) {
		t.Error("a post from another server may be replied to by somebody who is not signed in")
	}
	reader.Scopes = nil
	if CanReplyToPost(reader, note, &entities.ActorRelation{}) {
		t.Error("a post from another server may be replied to by a token that may not write posts")
	}

	// The same of what was made here is still allowed, so the test is of
	// the rule and not of an administrator who can do nothing.
	alice.URI, note.URI = "", ""
	if ok, _ := CanPostTo(admin, alice, nil, "text-service", "note"); !ok {
		t.Error("an administrator may not post on a profile here")
	}
	if !CanLike(admin, note, nil) {
		t.Error("an administrator may not like a post here")
	}
}
