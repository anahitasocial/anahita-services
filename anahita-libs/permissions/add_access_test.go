package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// ownerWith builds a profile with the given access level and add_permissions
// under text-service, the way the graph service's actor read returns one.
func ownerWith(id int64, access string, perms ...entities.AnahitaNodeAddPermission) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = id
	n.Access = access
	_ = n.SetFeatures([]*entities.AnahitaNodeFeature{{
		Service:        "text-service",
		Enabled:        true,
		AddPermissions: perms,
	}})
	return n
}

func perm(entity, access string) entities.AnahitaNodeAddPermission {
	return entities.AnahitaNodeAddPermission{Entity: entity, Access: access}
}

func TestAddAccessAllows(t *testing.T) {
	viewer := viewerWith(constants.PersonTypeRegistered)

	follower := &entities.ActorRelation{IsLeading: true}
	leader := &entities.ActorRelation{IsFollowing: true}
	mutual := &entities.ActorRelation{IsLeading: true, IsFollowing: true, IsMutual: true}
	admin := &entities.ActorRelation{HasAdmin: true}
	stranger := &entities.ActorRelation{}

	cases := []struct {
		name   string
		access string
		rel    *entities.ActorRelation
		want   bool
	}{
		{"registered admits a stranger", "registered", stranger, true},
		{"followers admits a follower", "followers", follower, true},
		{"followers refuses a leader", "followers", leader, false},
		{"leaders admits a leader", "leaders", leader, true},
		{"leaders refuses a follower", "leaders", follower, false},
		{"mutuals admits a mutual", "mutuals", mutual, true},
		{"mutuals refuses a one-way follower", "mutuals", follower, false},
		{"admins refuses a follower", "admins", follower, false},
		// An administrator meets every level, including ones nobody set.
		{"admins admits an admin", "admins", admin, true},
		{"unset admits an admin", "", admin, true},
		{"unset refuses a follower", "", follower, false},
		{"an unknown value refuses", "everyone", follower, false},
		{"a missing relationship is no relationship", "followers", nil, false},
	}

	for _, c := range cases {
		if got := AddAccessAllows(c.access, c.rel, viewer); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}

	guest := entities.NewViewer()
	if AddAccessAllows("registered", stranger, guest) {
		t.Error("registered admitted a guest")
	}
}

func TestCommentAccess(t *testing.T) {
	cases := []struct {
		name  string
		owner *entities.AnahitaNode
		want  string
	}{
		{"public, unset, defaults to registered", ownerWith(1, "public"), "registered"},
		{"public, followers", ownerWith(1, "public", perm("comment", "followers")), "followers"},
		{"registered, registered", ownerWith(1, "registered", perm("comment", "registered")), "registered"},
		// Only the two levels count; anything else stored falls back.
		{"public, admins falls back to registered", ownerWith(1, "public", perm("comment", "admins")), "registered"},
		// The profile's access overrides whatever is stored.
		{"followers profile is followers", ownerWith(1, "followers", perm("comment", "registered")), "followers"},
		{"mutuals profile is whoever can read", ownerWith(1, "mutuals", perm("comment", "registered")), ""},
		{"myself profile is whoever can read", ownerWith(1, "myself"), ""},
	}

	for _, c := range cases {
		if got := CommentAccess(c.owner, "text-service"); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
