package permissions

import (
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// groupInviting is a group in use whose "invite" permission is as given, or
// unset when it is "".
func groupInviting(access string, invite string) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = 50
	n.Type = "node.actor.group-service.group.v1"
	n.Access = access
	n.Enabled = true

	perms := []entities.AnahitaNodeAddPermission{perm("follower", "admins")}
	if invite != "" {
		perms = append(perms, perm("invite", invite))
	}
	_ = n.SetFeatures([]*entities.AnahitaNodeFeature{{
		Service:        "socialgraph-service",
		Enabled:        true,
		AddPermissions: perms,
	}})

	return n
}

func TestCanInvite(t *testing.T) {
	now := time.Now()
	follower := &entities.ActorRelation{IsLeading: true}
	admin := &entities.ActorRelation{HasAdmin: true}
	stranger := &entities.ActorRelation{}
	siteAdmin := viewerWith(constants.PersonTypeAdmin)

	gone := groupInviting("public", "followers")
	gone.ArchivedAt = &now
	off := groupInviting("public", "followers")
	off.Enabled = false

	cases := []struct {
		name   string
		viewer *entities.Viewer
		group  *entities.AnahitaNode
		rel    *entities.ActorRelation
		want   bool
	}{
		{"admins: a group admin", viewerAs(100), groupInviting("public", "admins"), admin, true},
		{"admins: a follower", viewerAs(100), groupInviting("public", "admins"), follower, false},
		{"followers: a follower", viewerAs(100), groupInviting("public", "followers"), follower, true},
		{"followers: a stranger", viewerAs(100), groupInviting("public", "followers"), stranger, false},
		// A group that has never said keeps it to its administrators.
		{"unset: a group admin", viewerAs(100), groupInviting("public", ""), admin, true},
		{"unset: a follower", viewerAs(100), groupInviting("public", ""), follower, false},
		{"a private group, by a follower where followers may", viewerAs(100), groupInviting("followers", "followers"), follower, true},
		{"a private group, by somebody who cannot see it", viewerAs(100), groupInviting("followers", "followers"), stranger, false},
		{"a site administrator", siteAdmin, groupInviting("followers", "admins"), nil, true},
		{"an archived group", viewerAs(100), gone, admin, false},
		{"a disabled group", viewerAs(100), off, admin, false},
		{"a guest", entities.NewViewer(), groupInviting("public", "followers"), nil, false},
		{"never to a person", viewerAs(100), profileWith("public"), admin, false},
	}

	for _, c := range cases {
		if got := CanInvite(c.viewer, c.group, c.rel); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSeeingAndWithdrawingInvites(t *testing.T) {
	group := groupInviting("public", "followers")
	follower := &entities.ActorRelation{IsLeading: true}
	admin := &entities.ActorRelation{HasAdmin: true}
	siteAdmin := viewerWith(constants.PersonTypeAdmin)

	cases := []struct {
		name         string
		viewer       *entities.Viewer
		rel          *entities.ActorRelation
		inviterID    int64
		wantSee      bool
		wantWithdraw bool
	}{
		{"a group admin", viewerAs(100), admin, 7, true, true},
		{"whoever invited", viewerAs(100), follower, 100, false, true},
		{"another follower", viewerAs(100), follower, 7, false, false},
		{"a site administrator", siteAdmin, nil, 7, true, true},
		{"a guest", entities.NewViewer(), nil, 0, false, false},
	}

	for _, c := range cases {
		if got := CanSeeInvites(c.viewer, group, c.rel); got != c.wantSee {
			t.Errorf("see, %s: got %v, want %v", c.name, got, c.wantSee)
		}
		if got := CanWithdrawInvite(c.viewer, group, c.rel, c.inviterID); got != c.wantWithdraw {
			t.Errorf("withdraw, %s: got %v, want %v", c.name, got, c.wantWithdraw)
		}
	}
}
