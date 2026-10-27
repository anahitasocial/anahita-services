package permissions

import (
	"testing"
	"time"

	"anahita-libs/data/entities"
)

func TestCanSeeLimitedProfile(t *testing.T) {
	now := time.Now()
	asked := func(change func(*entities.AnahitaNode)) *entities.AnahitaNode {
		n := personNode(followTargetID, "followers")
		n.Enabled = true
		n.AllowFollowRequest = true
		if change != nil {
			change(n)
		}
		return n
	}

	cases := []struct {
		name   string
		viewer *entities.Viewer
		actor  *entities.AnahitaNode
		rel    *entities.ActorRelation
		want   bool
	}{
		{"a profile that may be asked", viewerAs(100), asked(nil), nil, true},
		{"one that may not", viewerAs(100), asked(func(n *entities.AnahitaNode) { n.AllowFollowRequest = false }), nil, false},
		// An invitation has to be seen to be answered.
		{"one that may not, to somebody invited", viewerAs(100), asked(func(n *entities.AnahitaNode) { n.AllowFollowRequest = false }), &entities.ActorRelation{HasInvite: true}, true},
		{"an event open to its host's followers, to one of them", viewerAs(100), asked(func(n *entities.AnahitaNode) { n.AllowFollowRequest = false }), &entities.ActorRelation{ViaHost: true}, true},
		{"somebody invited and since blocked", viewerAs(100), asked(nil), &entities.ActorRelation{HasInvite: true, IsBlocking: true}, false},
		{"a visitor", entities.NewViewer(), asked(nil), nil, false},
		{"somebody it blocked", viewerAs(100), asked(nil), &entities.ActorRelation{IsBlocking: true}, false},
		{"a disabled profile", viewerAs(100), asked(func(n *entities.AnahitaNode) { n.Enabled = false }), nil, false},
		{"one on its way out", viewerAs(100), asked(func(n *entities.AnahitaNode) { n.DeletedAt = &now }), nil, false},
		{"an archived one", viewerAs(100), asked(func(n *entities.AnahitaNode) { n.ArchivedAt = &now }), nil, false},
		{"nothing", viewerAs(100), nil, nil, false},
	}

	for _, c := range cases {
		if got := CanSeeLimitedProfile(c.viewer, c.actor, c.rel); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
