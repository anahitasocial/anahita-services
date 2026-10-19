package handlers

import (
	"testing"

	an_entities "anahita-libs/data/entities"
	an_services "anahita-libs/data/services"
	"anahita-libs/event"

	"notification/data/services"
)

type fakeActors struct {
	an_services.ActorsGRPCInterface
}

func (f *fakeActors) Read(id int64, viewer *an_entities.Viewer) (*an_entities.AnahitaNode, error) {
	return &an_entities.AnahitaNode{ID: id, Type: "node.actor.person-service.person.v1"}, nil
}

type fakeNodes struct {
	an_services.GraphNodeGRPCInterface
	kept map[int64]*an_entities.AnahitaNode
}

func (f *fakeNodes) Read(id int64) (*an_entities.AnahitaNode, error) {
	return f.kept[id], nil
}

type countedSubscriptions struct {
	an_services.SubscriptionsGRPCInterface
	asked int
}

func (f *countedSubscriptions) BrowseSources(target *an_entities.AnahitaNode) ([]*an_entities.AnahitaNode, error) {
	f.asked++
	return nil, nil
}

// WHAT THIS GUARDS (roadmap row 101, part 3): a thread written in one go
// is told of once, by its first note. The notes after it tell nobody; a
// reply made on its own, or a thread's first note, tells as a reply does.
func TestAThreadIsToldOfOnce(t *testing.T) {
	const sisko, root, reply = 7, 100, 101
	for _, tc := range []struct {
		name  string
		part  int
		asked bool
	}{
		{"a reply on its own", 0, true},
		{"the first note of a thread", 1, true},
		{"a note after it", 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			subs := &countedSubscriptions{}
			h := &eventHandler{Env: &Env{Srv: &services.Services{
				ActorsGRPC: &fakeActors{},
				GraphNodeGRPC: &fakeNodes{kept: map[int64]*an_entities.AnahitaNode{
					root:  {ID: root, CreatedBy: sisko},
					reply: {ID: reply, RootID: root, ParentID: root, CreatedBy: sisko},
				}},
				SubscriptionsGRPC: subs,
			}}}
			payload := &event.EventPayload{SubjectID: sisko, ObjectID: reply, Part: tc.part}
			if err := h.eventFormatNodeMediumAdded(payload); err != nil {
				t.Fatal(err)
			}
			if asked := subs.asked > 0; asked != tc.asked {
				t.Errorf("who to tell was asked: %v, want %v", asked, tc.asked)
			}
		})
	}
}
