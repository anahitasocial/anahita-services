package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

const (
	followTargetID = 1
	followViewerID = 100
	followOtherID  = 200
)

func personNode(id int64, access string) *entities.AnahitaNode {
	n := profileWith(access)
	n.ID = id
	return n
}

func groupNode(access string, followerPerm string) *entities.AnahitaNode {
	n := groupWith(access, followerPerm)
	n.ID = followTargetID
	return n
}

func asksToBeFollowed(n *entities.AnahitaNode) *entities.AnahitaNode {
	n.AllowFollowRequest = true
	return n
}

func TestCanFollow(t *testing.T) {
	viewer := viewerAs(followViewerID)
	me := personNode(followViewerID, "public")
	other := personNode(followOtherID, "public")
	none := &entities.ActorRelation{}
	blocked := &entities.ActorRelation{IsBlocking: true}
	admin := &entities.ActorRelation{HasAdmin: true}

	cases := []struct {
		name string
		ctx  *FollowContext
		want bool
	}{
		{"yourself, a public person", &FollowContext{viewer, me, personNode(followTargetID, "public"), none, none}, true},
		{"yourself, a public group", &FollowContext{viewer, me, groupNode("public", "admins"), none, none}, true},
		{"yourself, a profile you cannot see", &FollowContext{viewer, me, personNode(followTargetID, "followers"), none, none}, false},
		{"yourself, somebody who blocked you", &FollowContext{viewer, me, personNode(followTargetID, "public"), blocked, blocked}, false},
		// The hole this closes: a stranger made to follow a stranger.
		{"somebody else, a person", &FollowContext{viewer, other, personNode(followTargetID, "public"), none, none}, false},
		{"somebody else, a person, as a site administrator", &FollowContext{viewerWith(constants.PersonTypeAdmin), other, personNode(followTargetID, "public"), none, none}, false},
		{"somebody else, your own profile", &FollowContext{viewer, other, me, none, none}, false},
		// Not even a group's administrators: people are invited, and choose.
		{"somebody else, a group you administer", &FollowContext{viewer, other, groupNode("public", "admins"), admin, none}, false},
		{"somebody else, a group, as a site administrator", &FollowContext{viewerWith(constants.PersonTypeAdmin), other, groupNode("public", "admins"), none, none}, true},
		{"somebody else, a group you only follow", &FollowContext{viewer, other, groupNode("public", "admins"), &entities.ActorRelation{IsLeading: true}, none}, false},
		{"somebody else who is blocked by the group", &FollowContext{viewer, other, groupNode("public", "admins"), admin, blocked}, false},
		{"a profile following itself", &FollowContext{viewer, me, me, none, none}, false},
		{"a guest", &FollowContext{entities.NewViewer(), me, personNode(followTargetID, "public"), none, none}, false},
		{"nothing", nil, false},
	}

	for _, c := range cases {
		if got := CanFollow(c.ctx); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCanRemoveFollower(t *testing.T) {
	viewer := viewerAs(followViewerID)
	me := personNode(followViewerID, "public")
	other := personNode(followOtherID, "public")
	none := &entities.ActorRelation{}
	admin := &entities.ActorRelation{HasAdmin: true}

	cases := []struct {
		name string
		ctx  *FollowContext
		want bool
	}{
		{"unfollowing", &FollowContext{viewer, me, personNode(followTargetID, "public"), none, none}, true},
		// Unfollowing does not depend on still being able to see it.
		{"unfollowing a profile you can no longer see", &FollowContext{viewer, me, personNode(followTargetID, "myself"), none, none}, true},
		{"removing a follower of your own", &FollowContext{viewer, other, me, none, none}, true},
		{"removing a follower of a group you administer", &FollowContext{viewer, other, groupNode("public", "admins"), admin, none}, true},
		{"a site administrator", &FollowContext{viewerWith(constants.PersonTypeAdmin), other, personNode(followTargetID, "public"), none, none}, true},
		// The hole this closes: anybody ending anybody's follow.
		{"somebody else's follow of a public person", &FollowContext{viewer, other, personNode(followTargetID, "public"), none, none}, false},
		{"somebody else's follow of a group you only follow", &FollowContext{viewer, other, groupNode("public", "followers"), &entities.ActorRelation{IsLeading: true}, none}, false},
		{"a guest", &FollowContext{entities.NewViewer(), me, personNode(followTargetID, "public"), none, none}, false},
	}

	for _, c := range cases {
		if got := CanRemoveFollower(c.ctx); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCanRequestToFollow(t *testing.T) {
	viewer := viewerAs(followViewerID)
	me := personNode(followViewerID, "public")
	other := personNode(followOtherID, "public")
	none := &entities.ActorRelation{}

	asked := func() *entities.AnahitaNode {
		return asksToBeFollowed(personNode(followTargetID, "followers"))
	}

	cases := []struct {
		name string
		ctx  *FollowContext
		want bool
	}{
		// The case it is for: nobody but its followers sees it, and it
		// has said it may be asked.
		{"a private profile that may be asked", &FollowContext{viewer, me, asked(), none, none}, true},
		{"a private profile that may not", &FollowContext{viewer, me, personNode(followTargetID, "followers"), none, none}, false},
		{"one you already follow", &FollowContext{viewer, me, asked(), none, &entities.ActorRelation{IsLeading: true}}, false},
		{"one that blocked you", &FollowContext{viewer, me, asked(), none, &entities.ActorRelation{IsBlocking: true}}, false},
		{"on somebody else's behalf", &FollowContext{viewer, other, asked(), none, none}, false},
		{"on somebody else's behalf, as a site administrator", &FollowContext{viewerWith(constants.PersonTypeAdmin), other, asked(), none, none}, false},
		{"a guest", &FollowContext{entities.NewViewer(), me, asked(), none, none}, false},
		// Public, and followed outright, whatever its column says.
		{"a Pulse", &FollowContext{viewer, me, func() *entities.AnahitaNode {
			pulse := asked()
			pulse.Type = constants.NodeActorPulseEntityType
			return pulse
		}(), none, none}, false},
	}

	for _, c := range cases {
		if got := CanRequestToFollow(c.ctx); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAnsweringAFollowRequest(t *testing.T) {
	viewer := viewerAs(followViewerID)
	me := personNode(followViewerID, "followers")
	other := personNode(followOtherID, "public")
	third := personNode(followTargetID, "followers")
	none := &entities.ActorRelation{}
	admin := &entities.ActorRelation{HasAdmin: true}

	cases := []struct {
		name       string
		ctx        *FollowContext
		wantAccept bool
		wantDrop   bool
	}{
		{"asked of you", &FollowContext{viewer, other, me, none, none}, true, true},
		{"asked of a group you administer", &FollowContext{viewer, other, groupNode("followers", "admins"), admin, none}, true, true},
		{"asked of a group you only follow", &FollowContext{viewer, other, groupNode("followers", "admins"), &entities.ActorRelation{IsLeading: true}, none}, false, false},
		// Whoever asked can take it back. They cannot accept it.
		{"your own request", &FollowContext{viewer, me, third, none, none}, false, true},
		{"between two other people", &FollowContext{viewer, other, third, none, none}, false, false},
		{"a site administrator", &FollowContext{viewerWith(constants.PersonTypeAdmin), other, third, none, none}, true, true},
		{"a guest", &FollowContext{entities.NewViewer(), other, third, none, none}, false, false},
	}

	for _, c := range cases {
		if got := CanAcceptFollowRequest(c.ctx); got != c.wantAccept {
			t.Errorf("accept, %s: got %v, want %v", c.name, got, c.wantAccept)
		}
		if got := CanDropFollowRequest(c.ctx); got != c.wantDrop {
			t.Errorf("drop, %s: got %v, want %v", c.name, got, c.wantDrop)
		}
	}
}
