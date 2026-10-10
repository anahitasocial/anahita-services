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

type fakeEvents struct {
	asked []int64
	err   error
}

func (f *fakeEvents) Read(ids []int64, personID int64) (map[int64]*entities.EventDetails, error) {
	f.asked = ids
	out := map[int64]*entities.EventDetails{}
	for _, id := range ids {
		out[id] = &entities.EventDetails{NodeID: id, OnlineURL: "https://meet.example/x", GoingCount: 3}
	}
	return out, f.err
}

func TestSetEventDetails(t *testing.T) {
	const event = "node.actor.actor-service.event.v1"
	const person = "node.actor.person-service.person.v1"

	nodes := []*entities.AnahitaNode{relNode(1, person), relNode(2, event), nil}
	events := &fakeEvents{}

	SetEventDetails(events, nodes, entities.NewViewer())

	if len(events.asked) != 1 || events.asked[0] != 2 {
		t.Errorf("asked about %v, want the event alone", events.asked)
	}
	if nodes[0].Event != nil {
		t.Error("a person was given event details")
	}
	if nodes[1].Event == nil || nodes[1].Event.GoingCount != 3 {
		t.Errorf("the event: %+v", nodes[1].Event)
	}
	// Never the way in from a list.
	if nodes[1].EventShowsOnlineURL {
		t.Error("a list shows where to join online")
	}

	// A failure leaves the list as it was, and nothing is asked without events.
	fresh := []*entities.AnahitaNode{relNode(2, event)}
	SetEventDetails(&fakeEvents{err: errors.New("down")}, fresh, nil)
	if fresh[0].Event != nil {
		t.Error("details after a failed read")
	}
	none := &fakeEvents{}
	SetEventDetails(none, []*entities.AnahitaNode{relNode(1, person)}, nil)
	if none.asked != nil {
		t.Error("asked with no events in the list")
	}
	SetEventDetails(nil, fresh, nil)
}
