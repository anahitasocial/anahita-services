package services

import (
	"actor/constants"
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	libs_services "anahita-libs/data/services"
)

// MentionsInterface keeps a node's mention edges in step with the @handles in
// its body.
//
// This was a repository writing `edges` with INSERT and DELETE from
// person-service. Edges belong to graph-grpc-service, and every operation it
// needed already existed on the edge and node RPCs — so the move needed no new
// service surface, only the four calls below.
type MentionsInterface interface {
	// Update reconciles the mention edges of a node against the aliases found
	// in its body: adds what is newly mentioned, removes what no longer is,
	// leaves the rest alone.
	Update(node MentionableInterface, aliases []string, viewer *entities.Viewer) error
}

// MentionableInterface is what Update needs of the node being mentioned FROM.
//
// Narrow on purpose: a note, an article, a photo and a person's profile body
// can all mention somebody, and none of them should have to be a particular
// type to do it.
type MentionableInterface interface {
	GetID() int64
	GetType() string
	// GetEditorID is the fallback for the edge's created_by when the caller has
	// no viewer id — see attribute().
	GetEditorID() int64
	GetAuthorID() int64
}

func NewMentions(nodes libs_services.GraphNodeGRPCInterface, edges libs_services.GraphEdgeGRPCInterface) MentionsInterface {
	return &Mentions{
		nodes:    nodes,
		edges:    edges,
		edgeType: constants.EdgeTagMentionType,
	}
}

type Mentions struct {
	nodes    libs_services.GraphNodeGRPCInterface
	edges    libs_services.GraphEdgeGRPCInterface
	edgeType string
}

func (m *Mentions) Update(node MentionableInterface, aliases []string, viewer *entities.Viewer) error {
	wanted, err := m.resolve(aliases)
	if err != nil {
		return err
	}

	current, err := m.current(node)
	if err != nil {
		return err
	}

	var added []*entities.AnahitaEdge
	for id := range wanted {
		if _, ok := current[id]; !ok {
			edge := entities.NewAnahitaEdge()
			edge.Type = m.edgeType
			edge.NodeAID = id
			edge.NodeAType = an_constants.NodeActorPersonEntityType
			edge.NodeBID = node.GetID()
			edge.NodeBType = node.GetType()
			edge.CreatedBy = attribute(node, viewer)
			edge.ModifiedBy = edge.CreatedBy
			added = append(added, edge)
		}
	}

	// One call for every new mention rather than one per mention. The old
	// repository looped, issuing an INSERT each time round.
	if len(added) > 0 {
		if _, err := m.edges.BatchAdd(added); err != nil {
			return err
		}
	}

	for id := range current {
		if _, ok := wanted[id]; ok {
			continue
		}

		req := libs_services.NewGraphEdgeRequest().BatchDeleteByFilters
		req.Types = []string{m.edgeType}
		req.NodeAID = id
		req.NodeAType = an_constants.NodeActorPersonEntityType
		req.NodeBID = node.GetID()
		req.NodeBType = node.GetType()

		if err := m.edges.BatchDeleteByFilters(req); err != nil {
			return err
		}
	}

	return nil
}

// resolve turns @handles into person ids.
//
// Unknown handles simply do not come back, which is the behaviour worth
// keeping: writing "@nobody" in a note should leave the note alone, not fail
// the edit.
func (m *Mentions) resolve(aliases []string) (map[int64]struct{}, error) {
	ids := make(map[int64]struct{})
	if len(aliases) == 0 {
		return ids, nil
	}

	req := libs_services.NewGraphNodeRequest().BatchReadByAliases
	req.Aliases = aliases
	req.NodeType = an_constants.NodeActorPersonEntityType

	nodes, err := m.nodes.BatchReadByAliases(req)
	if err != nil {
		return nil, err
	}

	for _, n := range nodes {
		ids[n.ID] = struct{}{}
	}

	return ids, nil
}

// current reads the mention edges a node already has.
func (m *Mentions) current(node MentionableInterface) (map[int64]struct{}, error) {
	req := libs_services.NewGraphEdgeRequest().BatchReadByFilters
	req.Types = []string{m.edgeType}
	req.NodeBID = node.GetID()

	edges, err := m.edges.BatchReadByFilters(req)
	if err != nil {
		return nil, err
	}

	ids := make(map[int64]struct{}, len(edges))
	for _, e := range edges {
		ids[e.NodeAID] = struct{}{}
	}

	return ids, nil
}

// attribute decides who an edge records as its creator.
//
// The viewer when there is one, then the node's editor, then its author. A
// mention edge is created by whoever wrote the body it was found in, and on the
// event path — where mentions are reconciled from a broker message rather than
// a request — the viewer id comes off the payload and can be absent.
//
// It has to resolve to something non-zero: the graph rejects an edge with no
// created_by, where the direct INSERT this replaced wrote a 0 without
// complaint. That is a real difference in behaviour, and the right way round —
// but it turns a silently attributed edge into a failed reconciliation unless
// the fallback exists.
func attribute(node MentionableInterface, viewer *entities.Viewer) int64 {
	if viewer != nil && viewer.ID > 0 {
		return viewer.ID
	}
	if id := node.GetEditorID(); id > 0 {
		return id
	}
	return node.GetAuthorID()
}
