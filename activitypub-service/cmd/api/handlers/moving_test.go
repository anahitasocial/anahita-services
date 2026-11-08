package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"
	"activitypub/remote"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/site"
)

// fakeGraphEdges keeps edges of any kind, as the graph does.
type fakeGraphEdges struct {
	services.GraphEdgeGRPCInterface
	edges []*entities.AnahitaEdge
}

func (f *fakeGraphEdges) matches(e *entities.AnahitaEdge, a, b int64, types []string) bool {
	return (a == 0 || e.NodeAID == a) && (b == 0 || e.NodeBID == b) && (len(types) == 0 || slices.Contains(types, e.Type))
}

func (f *fakeGraphEdges) BatchReadByFilters(req *services.GraphEdgeBatchReadByFiltersRequest) ([]*entities.AnahitaEdge, error) {
	var found []*entities.AnahitaEdge
	for _, e := range f.edges {
		if f.matches(e, req.NodeAID, req.NodeBID, req.Types) {
			found = append(found, e)
		}
	}
	return found, nil
}

func (f *fakeGraphEdges) BatchDeleteByFilters(req *services.GraphEdgeBatchDeleteByFiltersRequest) error {
	f.edges = slices.DeleteFunc(f.edges, func(e *entities.AnahitaEdge) bool {
		return f.matches(e, req.NodeAID, req.NodeBID, req.Types)
	})
	return nil
}

func (f *fakeGraphEdges) Delete(id int64) error {
	f.edges = slices.DeleteFunc(f.edges, func(e *entities.AnahitaEdge) bool { return e.ID == id })
	return nil
}

func (f *fakeGraphEdges) Add(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error) {
	// As the graph refuses one with nobody to have made it.
	if edge.Type == constants.EdgeMovedEntityType && edge.CreatedBy == 0 {
		return nil, errors.New("created_by is required")
	}
	edge.ID = int64(500 + len(f.edges))
	f.edges = append(f.edges, edge)
	return edge, nil
}

func (f *fakeGraphEdges) add(kind string, a, b int64) *entities.AnahitaEdge {
	edge, _ := f.Add(&entities.AnahitaEdge{Type: kind, NodeAID: a, NodeBID: b})
	return edge
}

// pairRelations answers from the edges: who follows, asked, blocks and
// mutes whom.
type pairRelations struct {
	services.SocialgraphGRPCInterface
	graph *fakeGraphEdges
}

func (f *pairRelations) has(kind string, a, b int64) bool {
	found, _ := f.graph.BatchReadByFilters(&services.GraphEdgeBatchReadByFiltersRequest{NodeAID: a, NodeBID: b, Types: []string{kind}})
	return len(found) > 0
}

// Read(a, b) is a's relation to b, read as socialgraph-service reads it:
// IsLeading when b follows a.
func (f *pairRelations) Read(a, b int64) (*entities.ActorRelation, error) {
	return &entities.ActorRelation{
		ActorAId: a, ActorBId: b,
		IsLeading:    f.has(constants.EdgeFollowEntityType, b, a),
		HasRequested: f.has(constants.EdgeFollowRequestEntityType, b, a),
		IsBlocking:   f.has(constants.EdgeBlockEntityType, a, b),
		IsMuting:     f.has(constants.EdgeMuteEntityType, a, b),
	}, nil
}

type graphFollows struct {
	services.SocialgraphFollowGRPCInterface
	graph *fakeGraphEdges
}

func (f *graphFollows) Add(req *services.SocialgraphFollowAddRequest) error {
	kind := constants.EdgeFollowEntityType
	if req.IsFollowRequest {
		kind = constants.EdgeFollowRequestEntityType
	}
	f.graph.add(kind, req.Source.ID, req.Target.ID)
	return nil
}

func (f *graphFollows) Delete(req *services.SocialgraphFollowDeleteRequest) error {
	kind := constants.EdgeFollowEntityType
	if req.IsFollowRequest {
		kind = constants.EdgeFollowRequestEntityType
	}
	return f.graph.BatchDeleteByFilters(&services.GraphEdgeBatchDeleteByFiltersRequest{
		NodeAID: req.Source.ID, NodeBID: req.Target.ID, Types: []string{kind},
	})
}

type graphBlocks struct {
	services.SocialgraphBlockGRPCInterface
	graph *fakeGraphEdges
}

func (f *graphBlocks) Add(source, target *entities.AnahitaNode, _ *entities.Viewer) error {
	f.graph.add(constants.EdgeBlockEntityType, source.ID, target.ID)
	return nil
}

type graphMutes struct {
	services.SocialgraphMuteGRPCInterface
	graph *fakeGraphEdges
}

func (f *graphMutes) Add(source, target *entities.AnahitaNode, _ *entities.Viewer, hide bool, endsAt time.Time) error {
	edge := f.graph.add(constants.EdgeMuteEntityType, source.ID, target.ID)
	edge.EndsAt = endsAt
	if !hide {
		edge.Meta = constants.MuteMetaNotifications
	}
	return nil
}

const naomiNew = "https://tycho.example/users/naomi"

// naomiAt describes the new account, naming the old one, Alice's, as its
// own, or not.
func naomiAt(alsoKnownAs []string, locked bool) string {
	raw, _ := json.Marshal(map[string]any{
		"id": naomiNew, "type": "Person", "preferredUsername": "naomi", "name": "Naomi Nagata",
		"inbox":                     naomiNew + "/inbox",
		"publicKey":                 map[string]any{"id": naomiNew + "#main-key", "owner": naomiNew, "publicKeyPem": remoteKey.PublicPEM},
		"manuallyApprovesFollowers": locked,
		"alsoKnownAs":               alsoKnownAs,
	})
	return string(raw)
}

type moveWorld struct {
	h     *Handlers
	graph *fakeGraphEdges
	told  []*event.EventPayload
	old   *entities.AnahitaNode
}

// Alice, on mastodon.example, followed by Amos, asked to be followed by
// Bobbie, blocked by Chrisjen, muted by Drummer for a week with her
// notifications kept, and by Holden for a mute that has ended.
func newMoveWorld(newDoc string) *moveWorld {
	graph := &fakeGraphEdges{}
	old := &entities.AnahitaNode{ID: 100, Type: constants.NodeActorActivityPubPersonEntityType, URI: aliceID, Alias: "alice@mastodon.example", Enabled: true, Access: "public"}
	person := func(id int64, alias string) *entities.AnahitaNode {
		return &entities.AnahitaNode{ID: id, Type: constants.NodeActorPersonEntityType, Alias: alias, Enabled: true, Access: "public"}
	}
	nodes := &fakeNodes{people: []*entities.AnahitaNode{
		old, person(1, "amos"), person(2, "bobbie"), person(3, "chrisjen"), person(4, "drummer"), person(5, "holden"),
	}}
	graph.add(constants.EdgeFollowEntityType, 1, 100)
	graph.add(constants.EdgeFollowRequestEntityType, 2, 100)
	graph.add(constants.EdgeBlockEntityType, 3, 100)
	week := graph.add(constants.EdgeMuteEntityType, 4, 100)
	week.EndsAt, week.Meta = time.Now().Add(7*24*time.Hour), constants.MuteMetaNotifications
	ended := graph.add(constants.EdgeMuteEntityType, 5, 100)
	ended.EndsAt = time.Now().Add(-time.Hour)

	w := &moveWorld{graph: graph, old: old}
	w.h = &Handlers{Env: &Env{
		Site:        site.Addresses{Domain: "example.org", WebURL: "https://example.org", APIURL: "https://api.example.org"},
		Nodes:       nodes,
		Actors:      &fakeActors{kept: map[int64]*repositories.RemoteActor{100: {NodeID: 100, URI: aliceID}}},
		Fetcher:     &fakeFetcher{pages: map[string]string{naomiNew: newDoc}},
		Edges:       graph,
		Relations:   &pairRelations{graph: graph},
		FollowEdges: &graphFollows{graph: graph},
		BlockEdges:  &graphBlocks{graph: graph},
		MuteEdges:   &graphMutes{graph: graph},
		Announce: func(payload *event.EventPayload, _ string) error {
			w.told = append(w.told, payload)
			return nil
		},
	}}
	return w
}

func (w *moveWorld) move(t *testing.T) {
	t.Helper()
	activity := &ap.Activity{Type: "Move"}
	activity.Actor, _ = json.Marshal(aliceID)
	activity.Object, _ = json.Marshal(aliceID)
	activity.Target, _ = json.Marshal(naomiNew)
	if err := w.h.moved(context.Background(), &remote.Actor{ID: aliceID}, activity); err != nil {
		t.Fatal(err)
	}
}

func (w *moveWorld) has(kind string, a, b int64) bool {
	found, _ := w.graph.BatchReadByFilters(&services.GraphEdgeBatchReadByFiltersRequest{NodeAID: a, NodeBID: b, Types: []string{kind}})
	return len(found) > 0
}

// THE RULE THIS GUARDS (roadmap row 124, part 2): a Move is believed only
// when the new account names the old one in its alsoKnownAs; then the old
// profile points to the new one, and follows, requests, blocks and lasting
// mutes carry over, each follower told once; a second Move does nothing
// twice.
func TestAMoveCarriesFollowsBlocksAndMutesOver(t *testing.T) {
	w := newMoveWorld(naomiAt([]string{aliceID}, false))
	w.move(t)

	newer, _ := w.h.Nodes.ReadByURI(naomiNew)
	if newer == nil {
		t.Fatal("the new account was not kept")
	}
	n := newer.ID
	if !w.has(constants.EdgeMovedEntityType, 100, n) {
		t.Errorf("the old profile does not point to the new one")
	}
	for _, c := range []struct {
		what string
		kind string
		a, b int64
		want bool
	}{
		{"Amos follows the new account", constants.EdgeFollowEntityType, 1, n, true},
		{"Amos no longer follows the old", constants.EdgeFollowEntityType, 1, 100, false},
		{"Bobbie follows the new account", constants.EdgeFollowEntityType, 2, n, true},
		{"Bobbie's request to the old is gone", constants.EdgeFollowRequestEntityType, 2, 100, false},
		{"Chrisjen blocks the new account", constants.EdgeBlockEntityType, 3, n, true},
		{"Chrisjen still blocks the old", constants.EdgeBlockEntityType, 3, 100, true},
		{"Drummer mutes the new account", constants.EdgeMuteEntityType, 4, n, true},
		{"Holden's ended mute is not carried", constants.EdgeMuteEntityType, 5, n, false},
	} {
		if got := w.has(c.kind, c.a, c.b); got != c.want {
			t.Errorf("%s: %v", c.what, got)
		}
	}
	mutes, _ := w.graph.BatchReadByFilters(&services.GraphEdgeBatchReadByFiltersRequest{NodeAID: 4, NodeBID: n, Types: []string{constants.EdgeMuteEntityType}})
	if len(mutes) == 1 && (mutes[0].Meta != constants.MuteMetaNotifications || mutes[0].EndsAt.IsZero()) {
		t.Errorf("Drummer's mute lost its end or its notifications: %+v", mutes[0])
	}
	if len(w.told) != 2 || w.told[0].Name != event.RemoteMovedFollowed || w.told[0].TargetID != 1 || w.told[0].ObjectID != n {
		t.Errorf("told: %+v", w.told)
	}

	// Heard again: nothing left to move, nobody told twice.
	edges := len(w.graph.edges)
	w.move(t)
	if len(w.graph.edges) != edges || len(w.told) != 2 {
		t.Errorf("a second Move did something: %d edges, %d told", len(w.graph.edges), len(w.told))
	}
}

func TestAMoveToAnAccountThatApprovesFollowersAsks(t *testing.T) {
	w := newMoveWorld(naomiAt([]string{aliceID}, true))
	w.move(t)
	newer, _ := w.h.Nodes.ReadByURI(naomiNew)
	if !w.has(constants.EdgeFollowRequestEntityType, 1, newer.ID) || w.has(constants.EdgeFollowEntityType, 1, newer.ID) {
		t.Errorf("Amos did not ask to follow the new account")
	}
	if len(w.told) == 0 || w.told[0].Name != event.RemoteMovedAsked {
		t.Errorf("told: %+v", w.told)
	}
}

func TestAMoveIsNotBelievedWithoutTheAlias(t *testing.T) {
	w := newMoveWorld(naomiAt(nil, false))
	w.move(t)
	if w.has(constants.EdgeMovedEntityType, 100, 0) || !w.has(constants.EdgeFollowEntityType, 1, 100) || len(w.told) > 0 {
		t.Errorf("a Move the new account does not confirm was believed")
	}
}

func TestMovingBackUndoesTheSignpost(t *testing.T) {
	w := newMoveWorld(naomiAt([]string{aliceID}, false))
	w.move(t)
	newer, _ := w.h.Nodes.ReadByURI(naomiNew)
	// Naomi moves back, from the new account to the old.
	if err := w.h.pointTo(newer, w.old); err != nil {
		t.Fatal(err)
	}
	if w.has(constants.EdgeMovedEntityType, 100, newer.ID) || !w.has(constants.EdgeMovedEntityType, newer.ID, 100) {
		t.Errorf("moving back did not turn the signpost round")
	}
}

func TestFollowsLeftOnAMovedProfileAreMovedByTheSweep(t *testing.T) {
	w := newMoveWorld(naomiAt([]string{aliceID}, false))
	w.move(t)
	newer, _ := w.h.Nodes.ReadByURI(naomiNew)
	// Somebody's follow, left behind by a restart.
	w.graph.add(constants.EdgeFollowEntityType, 5, 100)
	if err := w.h.movedLeft(100); err != nil {
		t.Fatal(err)
	}
	if w.has(constants.EdgeFollowEntityType, 5, 100) || !w.has(constants.EdgeFollowEntityType, 5, newer.ID) {
		t.Errorf("the follow left behind was not moved")
	}
}
