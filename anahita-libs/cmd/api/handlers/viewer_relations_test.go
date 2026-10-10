package handlers

import (
	"errors"
	"testing"

	"anahita-libs/data/entities"
)

type fakeRelations struct {
	asked []int64
	rels  map[int64]*entities.ActorRelation
	err   error
}

func (f *fakeRelations) Browse(ids []int64, viewerID int64) (map[int64]*entities.ActorRelation, error) {
	f.asked = ids
	return f.rels, f.err
}

func relNode(id int64, nodeType string) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = id
	n.Type = nodeType
	return n
}

func TestSetViewerRelations(t *testing.T) {
	const person = "node.actor.person-service.person.v1"
	const note = "node.medium.text-service.note.v1"

	viewer := entities.NewViewer()
	viewer.ID = 9
	viewer.Authenticated = true

	nodes := []*entities.AnahitaNode{relNode(1, person), relNode(2, note), relNode(3, person), nil}
	relations := &fakeRelations{rels: map[int64]*entities.ActorRelation{
		1: {IsFollowing: true},
		3: {IsLeading: true, IsFollowing: true},
	}}

	SetViewerRelations(relations, nodes, viewer)

	// Only the actors are asked about.
	if len(relations.asked) != 2 || relations.asked[0] != 1 || relations.asked[1] != 3 {
		t.Errorf("asked about %v, want 1 and 3", relations.asked)
	}
	// 1 follows the viewer and is not followed: "Follow back".
	if !nodes[0].IsFollowingViewer || nodes[0].IsLeadingViewer {
		t.Errorf("1: follows viewer %v, followed by viewer %v", nodes[0].IsFollowingViewer, nodes[0].IsLeadingViewer)
	}
	if nodes[1].IsFollowingViewer || nodes[1].IsLeadingViewer {
		t.Error("a note was marked")
	}
	if !nodes[2].IsFollowingViewer || !nodes[2].IsLeadingViewer {
		t.Error("3 and the viewer follow each other, and it does not say so")
	}

	// A visitor is not asked about, and a failure leaves the list as it was.
	asked := &fakeRelations{}
	SetViewerRelations(asked, nodes, entities.NewViewer())
	if asked.asked != nil {
		t.Error("asked on behalf of a visitor")
	}
	fresh := []*entities.AnahitaNode{relNode(1, person)}
	SetViewerRelations(&fakeRelations{err: errors.New("down")}, fresh, viewer)
	if fresh[0].IsFollowingViewer || fresh[0].IsLeadingViewer {
		t.Error("marked after a failed read")
	}
	SetViewerRelations(nil, fresh, viewer)
}
