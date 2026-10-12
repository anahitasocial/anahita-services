package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// THE RULE THIS GUARDS: what came from another server is looked at here and
// not acted on. Nobody posts on such a profile, changes such a post, or
// likes or answers one, a site administrator included: the first two are
// not this installation's to do, and the last two would never reach
// whoever they were meant for.
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
	if CanReplyToPost(admin, note, nil) {
		t.Error("a post from another server may be replied to")
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
