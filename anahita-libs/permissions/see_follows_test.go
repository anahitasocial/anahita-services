package permissions

import (
	"testing"

	"anahita-libs/data/entities"
)

// THE RULE THIS GUARDS (roadmap row 151): a profile's followers and whom it
// follows are seen by whoever may see the profile, narrowed by its own
// choice, never widened by it; and always by itself and its admins.
func TestCanSeeFollows(t *testing.T) {
	profile := func(access, graph string) *entities.AnahitaNode {
		n := personNode(followTargetID, access)
		n.GraphAccess = graph
		return n
	}
	follower := &entities.ActorRelation{IsLeading: true}
	mutual := &entities.ActorRelation{IsLeading: true, IsFollowing: true, IsMutual: true}

	cases := []struct {
		name   string
		viewer *entities.Viewer
		actor  *entities.AnahitaNode
		rel    *entities.ActorRelation
		want   bool
	}{
		{"as the profile, public", entities.NewViewer(), profile("public", ""), nil, true},
		{"signed in only, to a visitor", entities.NewViewer(), profile("public", "registered"), nil, false},
		{"signed in only, to somebody signed in", viewerAs(100), profile("public", "registered"), nil, true},
		{"followers only, to a stranger", viewerAs(100), profile("public", "followers"), nil, false},
		{"followers only, to a follower", viewerAs(100), profile("public", "followers"), follower, true},
		{"mutuals only, to a follower", viewerAs(100), profile("public", "mutuals"), follower, false},
		{"mutuals only, to a mutual", viewerAs(100), profile("public", "mutuals"), mutual, true},
		{"only me, to a mutual", viewerAs(100), profile("public", "admins"), mutual, false},
		{"only me, to itself", viewerAs(followTargetID), profile("public", "admins"), nil, true},
		{"never wider than the profile", viewerAs(100), profile("followers", "public"), nil, false},
		{"not to somebody it blocked", viewerAs(100), profile("public", ""), &entities.ActorRelation{IsBlocking: true}, false},
		{"its admins", viewerAs(100), profile("public", "admins"), &entities.ActorRelation{HasAdmin: true}, true},
	}

	for _, c := range cases {
		ctx := &ActorAccessContext{Viewer: c.viewer, Actor: c.actor, ActorViewerRel: c.rel}
		if ctx.ActorViewerRel == nil {
			ctx.ActorViewerRel = &entities.ActorRelation{}
		}
		if got := CanSeeFollows(ctx); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
