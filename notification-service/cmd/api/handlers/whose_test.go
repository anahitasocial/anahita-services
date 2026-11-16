package handlers

import (
	"sort"
	"testing"

	"anahita-libs/constants"
	an_entities "anahita-libs/data/entities"
	an_services "anahita-libs/data/services"
	"anahita-libs/event"
	"notification/data/repositories"
	"notification/data/services"
)

// The graph, as far as telling goes: what each node is, all of it visible.
type whoseGraph struct {
	an_services.GraphNodeGRPCInterface
	nodes map[int64]*an_entities.AnahitaNode
}

func (g *whoseGraph) Read(id int64) (*an_entities.AnahitaNode, error) {
	if node, ok := g.nodes[id]; ok {
		return node, nil
	}
	return &an_entities.AnahitaNode{}, nil
}

func (g *whoseGraph) FilterVisible(ids []int64, viewer *an_entities.Viewer) ([]int64, error) {
	return ids, nil
}

// Who is subscribed to each post, and who took part since.
type whoseSubscriptions struct {
	an_services.SubscriptionsGRPCInterface
	of    map[int64][]int64
	added [][2]int64
}

func (s *whoseSubscriptions) BrowseSources(target *an_entities.AnahitaNode) ([]*an_entities.AnahitaNode, error) {
	nodes := []*an_entities.AnahitaNode{}
	for _, id := range s.of[target.ID] {
		nodes = append(nodes, &an_entities.AnahitaNode{ID: id, Type: constants.NodeActorPersonEntityType})
	}
	return nodes, nil
}

func (s *whoseSubscriptions) Add(source, target *an_entities.AnahitaNode, viewer *an_entities.Viewer) error {
	s.added = append(s.added, [2]int64{source.ID, target.ID})
	return nil
}

// Who follows whom: follows[a][b] is a following b.
type whoseRelations struct {
	an_services.SocialgraphGRPCInterface
	follows map[int64]map[int64]bool
}

func (r *whoseRelations) Browse(as []int64, b int64) (map[int64]*an_entities.ActorRelation, error) {
	rels := map[int64]*an_entities.ActorRelation{}
	for _, a := range as {
		rels[a] = &an_entities.ActorRelation{ActorAId: a, ActorBId: b, IsFollowing: r.follows[a][b]}
	}
	return rels, nil
}

func (r *whoseRelations) Read(a, b int64) (*an_entities.ActorRelation, error) {
	return &an_entities.ActorRelation{ActorAId: a, ActorBId: b}, nil
}

type whoseAdmins struct {
	an_services.ActorAdminsGRPCInterface
	of map[int64][]int64
}

func (a *whoseAdmins) Browse(actorID int64) ([]*an_entities.AnahitaNode, error) {
	nodes := []*an_entities.AnahitaNode{}
	for _, id := range a.of[actorID] {
		nodes = append(nodes, &an_entities.AnahitaNode{ID: id, Type: constants.NodeActorPersonEntityType})
	}
	return nodes, nil
}

func toldTo(kept *keptNotifications) []int64 {
	ids := []int64{}
	for _, n := range kept.made {
		ids = append(ids, n.RecipientID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// THE RULE THIS GUARDS (roadmap row 146): what is yours, or your
// channel's, tells you of everyone; what you take part in tells you of the
// people you follow. A like tells the post's author, or a channel's admins,
// whoever liked; and a subscriber only when they follow whoever liked.
func TestWhatIsYoursAndWhatYouTakePartIn(t *testing.T) {
	const (
		naomi, amos, alex, bobbie = 1, 2, 3, 9
		post, rocinante, theirs   = 50, 60, 70
		holden, drummer           = 11, 12
	)
	person := constants.NodeActorPersonEntityType
	world := func() (*eventHandler, *keptNotifications) {
		kept := &keptNotifications{}
		return &eventHandler{Env: &Env{
			Srv: &services.Services{
				ActorsGRPC: &fakeActors{},
				GraphNodeGRPC: &whoseGraph{nodes: map[int64]*an_entities.AnahitaNode{
					naomi:     {ID: naomi, Type: person},
					rocinante: {ID: rocinante, Type: constants.NodeActorWebsiteEntityType},
					post:      {ID: post, CreatedBy: naomi, Type: constants.NodeMediumNoteEntityType},
					theirs:    {ID: theirs, CreatedBy: rocinante, Type: constants.NodeMediumNoteEntityType},
				}},
				// Amos follows Bobbie, who liked; Alex does not.
				SubscriptionsGRPC: &whoseSubscriptions{of: map[int64][]int64{post: {amos, alex}}},
				SocialgraphGRPC:   &whoseRelations{follows: map[int64]map[int64]bool{amos: {bobbie: true}}},
				ActorAdminsGRPC:   &whoseAdmins{of: map[int64][]int64{rocinante: {holden, drummer}}},
			},
			Repos: &repositories.Repositories{Notifications: kept},
		}}, kept
	}

	h, kept := world()
	if err := h.eventFormatEdgeVoteupAdded(&event.EventPayload{SourceService: "like-service", SubjectID: bobbie, OwnerID: naomi, ObjectID: post, TargetID: naomi}); err != nil {
		t.Fatal(err)
	}
	if got := toldTo(kept); len(got) != 2 || got[0] != naomi || got[1] != amos {
		t.Errorf("a like on Naomi's post told %v, want Naomi and Amos, not Alex", got)
	}

	h, kept = world()
	if err := h.eventFormatEdgeVoteupAdded(&event.EventPayload{SourceService: "like-service", SubjectID: bobbie, OwnerID: rocinante, ObjectID: theirs, TargetID: rocinante}); err != nil {
		t.Fatal(err)
	}
	if got := toldTo(kept); len(got) != 2 || got[0] != holden || got[1] != drummer {
		t.Errorf("a like on a channel's post told %v, want its admins", got)
	}
}
