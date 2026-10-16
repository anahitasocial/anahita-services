package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

func TestWhoPinsAPost(t *testing.T) {
	// A note somebody (authorID) wrote in a group (ownerID).
	inGroup := postOn(ownerWith(ownerID, "public"), "public")

	var (
		nobody = &entities.ActorRelation{}
		admin  = &entities.ActorRelation{HasAdmin: true}
		member = &entities.ActorRelation{IsLeading: true}
	)

	cases := []struct {
		name   string
		viewer *entities.Viewer
		rel    *entities.ActorRelation
		want   bool
	}{
		{"an administrator of the group", replyViewer(bystanderID, constants.PersonTypeRegistered), admin, true},
		{"the profile itself", replyViewer(ownerID, constants.PersonTypeRegistered), nobody, true},
		{"a site administrator", replyViewer(bystanderID, constants.PersonTypeAdmin), nobody, true},
		// Wrote it, and does not run the place it is in.
		{"the post's author", replyViewer(authorID, constants.PersonTypeRegistered), member, false},
		{"a follower of the group", replyViewer(bystanderID, constants.PersonTypeRegistered), member, false},
		{"a token that may not write posts", replyViewer(ownerID, constants.PersonTypeRegistered, constants.OAuthScopeLikesWrite), nobody, false},
		{"a visitor", entities.NewViewer(), nil, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CanPin(c.viewer, inGroup, c.rel); got != c.want {
				t.Errorf("CanPin = %v, want %v", got, c.want)
			}
		})
	}

	// In feeds and lists: the profile and its administrators, not a site
	// administrator.
	if !RunsProfile(replyViewer(bystanderID, constants.PersonTypeRegistered), ownerID, admin) ||
		!RunsProfile(replyViewer(ownerID, constants.PersonTypeRegistered), ownerID, nobody) ||
		RunsProfile(replyViewer(bystanderID, constants.PersonTypeAdmin), ownerID, nobody) {
		t.Error("RunsProfile is not whoever runs the profile")
	}

	// Never a reply, whoever asks.
	reply := postOn(ownerWith(ownerID, "public"), "")
	reply.RootID = 99
	if CanPin(replyViewer(ownerID, constants.PersonTypeAdmin), reply, admin) {
		t.Error("a reply was pinned")
	}
}
