package services

import (
	"actor/constants"
	"anahita-libs/cmd/api/permissions"
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	libs_services "anahita-libs/data/services"
)

// MentionsInterface keeps a node's mention edges in step with the profiles
// its text names.
//
// A mention is a post naming a profile (the user, 2026-10-17): a person, a
// group or an event, here or elsewhere, by its @handle or by its address on a
// line of its own, embedded. A post naming a post is a citation, and is
// text-service's to keep (handlers/embeds.go).
type MentionsInterface interface {
	// Update reconciles the mention edges of a node against the aliases
	// its text names: adds what is newly named and may be, removes what no
	// longer is, leaves the rest alone. It answers with the edges it added,
	// for whoever is to be told.
	Update(node MentionableInterface, aliases []string, viewer *entities.Viewer) ([]*entities.AnahitaEdge, error)
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

func NewMentions(nodes libs_services.GraphNodeGRPCInterface, edges libs_services.GraphEdgeGRPCInterface, socialgraph libs_services.SocialgraphGRPCInterface) MentionsInterface {
	return &Mentions{
		nodes:       nodes,
		edges:       edges,
		socialgraph: socialgraph,
		edgeType:    constants.EdgeTagMentionType,
	}
}

type Mentions struct {
	nodes       libs_services.GraphNodeGRPCInterface
	edges       libs_services.GraphEdgeGRPCInterface
	socialgraph libs_services.SocialgraphGRPCInterface
	edgeType    string
}

func (m *Mentions) Update(node MentionableInterface, aliases []string, viewer *entities.Viewer) ([]*entities.AnahitaEdge, error) {
	named, err := m.resolve(aliases)
	if err != nil {
		return nil, err
	}
	wanted, err := m.allowed(node, named)
	if err != nil {
		return nil, err
	}

	current, err := m.current(node)
	if err != nil {
		return nil, err
	}

	var added []*entities.AnahitaEdge
	for id, kind := range wanted {
		if _, ok := current[id]; !ok {
			edge := entities.NewAnahitaEdge()
			edge.Type = m.edgeType
			edge.NodeAID = id
			edge.NodeAType = kind
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
			return nil, err
		}
	}

	for id, kind := range current {
		if _, ok := wanted[id]; ok {
			continue
		}

		req := libs_services.NewGraphEdgeRequest().BatchDeleteByFilters
		req.Types = []string{m.edgeType}
		req.NodeAID = id
		req.NodeAType = kind
		req.NodeBID = node.GetID()
		req.NodeBType = node.GetType()

		if err := m.edges.BatchDeleteByFilters(req); err != nil {
			return nil, err
		}
	}

	return added, nil
}

// resolve turns aliases into the profiles they are, of every kind a post can
// name (constants.ProfileTypes).
//
// Unknown handles simply do not come back, which is the behaviour worth
// keeping: writing "@nobody" in a note should leave the note alone, not fail
// the edit.
func (m *Mentions) resolve(aliases []string) (map[int64]*entities.AnahitaNode, error) {
	found := make(map[int64]*entities.AnahitaNode)
	if len(aliases) == 0 {
		return found, nil
	}

	// One kind at a time: an alias is looked up within a type, and an empty
	// type matches nothing.
	for _, kind := range an_constants.ProfileTypes {
		req := libs_services.NewGraphNodeRequest().BatchReadByAliases
		req.Aliases = aliases
		req.NodeType = kind

		nodes, err := m.nodes.BatchReadByAliases(req)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			found[n.ID] = n
		}
	}

	return found, nil
}

// allowed is the profiles named that may be (permissions.CanMention), by
// id, with their type. A person who said nobody, or not this author, may
// be, or who blocks them, is left out: no edge, and so nobody is told.
func (m *Mentions) allowed(node MentionableInterface, named map[int64]*entities.AnahitaNode) (map[int64]string, error) {
	wanted := make(map[int64]string, len(named))
	if len(named) == 0 {
		return wanted, nil
	}

	author := entities.NewViewer()
	author.ID = node.GetAuthorID()

	var people []int64
	for id, n := range named {
		if n.Type == an_constants.NodeActorPersonEntityType && id != author.ID {
			people = append(people, id)
		}
	}
	relations := map[int64]*entities.ActorRelation{}
	if len(people) > 0 && author.ID > 0 {
		read, err := m.socialgraph.Browse(people, author.ID)
		if err != nil {
			return nil, err
		}
		relations = read
	}

	for id, n := range named {
		if permissions.CanMention(n, author, relations[id]) {
			wanted[id] = n.Type
		}
	}
	return wanted, nil
}

// current reads the mention edges a node already has: who it names, with
// their type.
func (m *Mentions) current(node MentionableInterface) (map[int64]string, error) {
	req := libs_services.NewGraphEdgeRequest().BatchReadByFilters
	req.Types = []string{m.edgeType}
	req.NodeBID = node.GetID()

	edges, err := m.edges.BatchReadByFilters(req)
	if err != nil {
		return nil, err
	}

	ids := make(map[int64]string, len(edges))
	for _, e := range edges {
		ids[e.NodeAID] = e.NodeAType
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
