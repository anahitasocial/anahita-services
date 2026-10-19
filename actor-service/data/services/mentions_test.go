package services

import (
	"testing"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	libs_services "anahita-libs/data/services"
)

type fakeNodes struct {
	libs_services.GraphNodeGRPCInterface
	byAlias map[string]int64
	// The kind of each, a person unless said.
	kinds map[int64]string
	// Who lets whom mention them: their `mention` permission.
	lets  map[int64]string
	asked []string
}

func (f *fakeNodes) BatchReadByAliases(req *libs_services.GraphNodeBatchReadByAliasesRequest) ([]*entities.AnahitaNode, error) {
	f.asked = req.Aliases
	var out []*entities.AnahitaNode
	for _, a := range req.Aliases {
		id, ok := f.byAlias[a]
		if !ok {
			continue
		}
		kind := f.kinds[id]
		if kind == "" {
			kind = an_constants.NodeActorPersonEntityType
		}
		if kind != req.NodeType {
			continue
		}
		n := entities.NewAnahitaNode()
		n.ID = id
		n.Type = kind
		if access := f.lets[id]; access != "" {
			_ = n.SetFeatures([]*entities.AnahitaNodeFeature{{
				Service: an_constants.MentionPermissionService,
				Enabled: true,
				AddPermissions: []entities.AnahitaNodeAddPermission{
					{Entity: an_constants.MentionPermission, Access: access},
				},
			}})
		}
		out = append(out, n)
	}
	return out, nil
}

type fakeSocialgraph struct {
	libs_services.SocialgraphGRPCInterface
	blocking map[int64]bool
}

func (f *fakeSocialgraph) Browse(ids []int64, author int64) (map[int64]*entities.ActorRelation, error) {
	out := map[int64]*entities.ActorRelation{}
	for _, id := range ids {
		out[id] = &entities.ActorRelation{ActorAId: id, ActorBId: author, IsBlocking: f.blocking[id]}
	}
	return out, nil
}

type fakeEdges struct {
	libs_services.GraphEdgeGRPCInterface
	existing []int64
	added    []*entities.AnahitaEdge
	deleted  []int64
}

func (f *fakeEdges) BatchReadByFilters(req *libs_services.GraphEdgeBatchReadByFiltersRequest) ([]*entities.AnahitaEdge, error) {
	var out []*entities.AnahitaEdge
	for _, id := range f.existing {
		e := entities.NewAnahitaEdge()
		e.NodeAID = id
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeEdges) BatchAdd(edges []*entities.AnahitaEdge) ([]*entities.AnahitaEdge, error) {
	f.added = append(f.added, edges...)
	return edges, nil
}

func (f *fakeEdges) BatchDeleteByFilters(req *libs_services.GraphEdgeBatchDeleteByFiltersRequest) error {
	f.deleted = append(f.deleted, req.NodeAID)
	return nil
}

type mentionable struct {
	id       int64
	editorID int64
	authorID int64
}

func (m mentionable) GetID() int64       { return m.id }
func (m mentionable) GetType() string    { return "node.medium.text-service.note.v1" }
func (m mentionable) GetEditorID() int64 { return m.editorID }
func (m mentionable) GetAuthorID() int64 { return m.authorID }

// Update is a reconciliation, not a rewrite: it adds what is newly mentioned,
// removes what no longer is, and leaves everything else alone.
//
// Deleting all of them and re-adding would be simpler to write and wrong — the
// edges carry created_at, so rewriting them would redate every mention on every
// edit of the body.
func TestMentionsUpdateReconciles(t *testing.T) {
	nodes := &fakeNodes{byAlias: map[string]int64{"ada": 1, "grace": 2}}
	// 1 is already mentioned and stays; 3 is mentioned and no longer in the body.
	edges := &fakeEdges{existing: []int64{1, 3}}

	m := NewMentions(nodes, edges, &fakeSocialgraph{})
	_, removed, err := m.Update(mentionable{id: 99, authorID: 9}, []string{"ada", "grace"}, entities.NewViewer())
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	// Said, so that the news of being named goes too.
	if len(removed) != 1 || removed[0] != 3 {
		t.Errorf("expected 3 to be answered as removed, got %v", removed)
	}

	if len(edges.added) != 1 || edges.added[0].NodeAID != 2 {
		t.Errorf("expected only grace (2) to be added, got %+v", edges.added)
	}
	if len(edges.deleted) != 1 || edges.deleted[0] != 3 {
		t.Errorf("expected only 3 to be removed, got %v", edges.deleted)
	}
}

// An unknown handle is not an error. Writing "@nobody" in a note should leave
// the note alone rather than fail the edit.
func TestMentionsIgnoresUnknownHandles(t *testing.T) {
	nodes := &fakeNodes{byAlias: map[string]int64{"ada": 1}}
	edges := &fakeEdges{}

	m := NewMentions(nodes, edges, &fakeSocialgraph{})
	if _, _, err := m.Update(mentionable{id: 99, authorID: 9}, []string{"ada", "nobody"}, entities.NewViewer()); err != nil {
		t.Fatalf("update: %v", err)
	}

	if len(edges.added) != 1 || edges.added[0].NodeAID != 1 {
		t.Errorf("expected only the known handle to be linked, got %+v", edges.added)
	}
}

// No aliases means no lookup at all, rather than a read with an empty IN clause.
func TestMentionsSkipsTheLookupWhenNothingIsMentioned(t *testing.T) {
	nodes := &fakeNodes{byAlias: map[string]int64{"ada": 1}}
	edges := &fakeEdges{existing: []int64{1}}

	m := NewMentions(nodes, edges, &fakeSocialgraph{})
	if _, _, err := m.Update(mentionable{id: 99, authorID: 9}, nil, entities.NewViewer()); err != nil {
		t.Fatalf("update: %v", err)
	}

	if nodes.asked != nil {
		t.Errorf("looked up aliases when there were none: %v", nodes.asked)
	}
	// The body mentions nobody, so the existing mention has to go.
	if len(edges.deleted) != 1 || edges.deleted[0] != 1 {
		t.Errorf("expected the stale mention to be removed, got %v", edges.deleted)
	}
}

// The edge has to record who created it: the graph rejects created_by 0, where
// the direct INSERT this replaced wrote one without complaint.
//
// On the event path the viewer id comes off a broker payload and can be
// absent, so it falls back to whoever wrote the body — editor first, then
// author.
func TestMentionsAttributeTheEdge(t *testing.T) {
	for _, tc := range []struct {
		name     string
		viewerID int64
		node     mentionable
		want     int64
	}{
		{"the viewer when there is one", 7, mentionable{id: 99, editorID: 3, authorID: 4}, 7},
		{"the editor when there is not", 0, mentionable{id: 99, editorID: 3, authorID: 4}, 3},
		{"the author when there is neither", 0, mentionable{id: 99, authorID: 4}, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes := &fakeNodes{byAlias: map[string]int64{"ada": 1}}
			edges := &fakeEdges{}

			viewer := entities.NewViewer()
			viewer.ID = tc.viewerID

			if _, _, err := NewMentions(nodes, edges, &fakeSocialgraph{}).Update(tc.node, []string{"ada"}, viewer); err != nil {
				t.Fatalf("update: %v", err)
			}
			if len(edges.added) != 1 {
				t.Fatalf("expected one edge, got %d", len(edges.added))
			}
			if got := edges.added[0].CreatedBy; got != tc.want {
				t.Errorf("created_by = %d, want %d", got, tc.want)
			}
		})
	}
}

// Who may be named (permissions.CanMention): a person who lets nobody, or
// blocks the author, is no edge; a group always is, with its own type.
func TestMentionsKeepOnlyWhoMayBeNamed(t *testing.T) {
	nodes := &fakeNodes{
		byAlias: map[string]int64{"ada": 1, "grace": 2, "club": 3, "eve": 4},
		kinds:   map[int64]string{3: an_constants.NodeActorGroupEntityType},
		lets:    map[int64]string{2: "admins"},
	}
	edges := &fakeEdges{}
	social := &fakeSocialgraph{blocking: map[int64]bool{4: true}}

	m := NewMentions(nodes, edges, social)
	added, _, err := m.Update(mentionable{id: 99, authorID: 9}, []string{"ada", "grace", "club", "eve"}, entities.NewViewer())
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	kept := map[int64]string{}
	for _, e := range added {
		kept[e.NodeAID] = e.NodeAType
	}
	if len(kept) != 2 || kept[1] != an_constants.NodeActorPersonEntityType || kept[3] != an_constants.NodeActorGroupEntityType {
		t.Errorf("expected ada and the club, got %v", kept)
	}
}
