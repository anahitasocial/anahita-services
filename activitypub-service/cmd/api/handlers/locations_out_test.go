package handlers

import (
	"encoding/json"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
)

// The locations a post was tagged with, as the graph would answer.
type fakeLocations struct {
	of    map[int64][]*entities.AnahitaNode
	asked []*services.GraphBrowseConnectionNodesRequest
}

func (f *fakeLocations) BrowseConnectionNodes(req *services.GraphBrowseConnectionNodesRequest) ([]*entities.AnahitaNode, uint64, error) {
	f.asked = append(f.asked, req)
	found := f.of[req.NodeId]
	return found, uint64(len(found)), nil
}

func location(id int64, name, alias string, lat, lng float64) *entities.AnahitaNode {
	node := entities.NewAnahitaNode()
	node.ID = id
	node.Type = constants.NodeBaseLocationEntityType
	node.Name = name
	node.Alias = alias
	node.GeoLatitude = lat
	node.GeoLongitude = lng
	return node
}

func relocatedEvent(id int64, objectType string) *event.EventPayload {
	return &event.EventPayload{Name: "edge.location.location.added.v1", ObjectID: id, ObjectType: objectType}
}

// THE RULE THIS GUARDS (the user, 2026-10-19; roadmap row 82, part 3b): a
// post goes out with the locations its author chose for it, the first as its
// `location`, as servers that read one expect, and every one as a `Place` in
// its `tag`, each with its point and the page it is read at here. A post
// with none carries neither.
func TestAPostGoesOutWithItsLocations(t *testing.T) {
	w := notesWorld()
	places := &fakeLocations{of: map[int64][]*entities.AnahitaNode{noteID: {
		location(31, "Promenade", "promenade", 49.2827, -123.1207),
		location(32, "Ops", "ops", 49.2869, -123.143),
	}}}
	w.h.Locations = places

	if err := w.h.HandleEvent(made(noteID)); err != nil {
		t.Fatal(err)
	}
	if len(w.queue.sent) == 0 {
		t.Fatal("nothing sent")
	}
	object, _ := w.queue.sent[0].payload["object"].(map[string]any)

	where, _ := json.Marshal(object["location"])
	if string(where) != `{"latitude":49.2827,"longitude":-123.1207,"name":"Promenade","type":"Place","url":"https://example.org/locations/31-promenade/"}` {
		t.Errorf("location %s", where)
	}
	tags, _ := json.Marshal(object["tag"])
	want := `[{"href":"https://example.org/tags/garden","name":"#garden","type":"Hashtag"},` +
		`{"href":"https://example.org/locations/31-promenade/","latitude":49.2827,"longitude":-123.1207,"name":"Promenade","type":"Place"},` +
		`{"href":"https://example.org/locations/32-ops/","latitude":49.2869,"longitude":-123.143,"name":"Ops","type":"Place"}]`
	if string(tags) != want {
		t.Errorf("tags %s\nwant %s", tags, want)
	}
	if len(places.asked) == 0 || places.asked[0].EdgeType != constants.EdgeTagLocationType ||
		places.asked[0].Direction != constants.GraphQueryDirectionInbound || places.asked[0].OrderingDir != constants.DirAsc {
		t.Errorf("asked the graph %+v", places.asked)
	}

	// None: neither.
	w = notesWorld()
	w.h.Locations = &fakeLocations{}
	_ = w.h.HandleEvent(made(noteID))
	object, _ = w.queue.sent[0].payload["object"].(map[string]any)
	if _, has := object["location"]; has {
		t.Errorf("a post with no location sent one: %v", object["location"])
	}
}

// A location added to a post, or taken off it, from its page: the post's
// own dates do not change, but it goes again, as an Update said to be
// changed now, once for each such change. A topic, which stays here, does
// not.
func TestAPostsLocationsChangedGoesAgain(t *testing.T) {
	w := notesWorld()
	places := &fakeLocations{of: map[int64][]*entities.AnahitaNode{}}
	w.h.Locations = places
	_ = w.h.HandleEvent(made(noteID))
	w.queue.sent = nil

	places.of[noteID] = []*entities.AnahitaNode{location(31, "Promenade", "promenade", 49.2827, -123.1207)}
	if err := w.h.HandleEvent(relocatedEvent(noteID, constants.NodeMediumNoteEntityType)); err != nil {
		t.Fatal(err)
	}
	if len(w.queue.sent) != 2 || w.queue.sent[0].payload["type"] != "Update" {
		t.Fatalf("after a location was added: %+v", w.queue.sent)
	}
	object, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if object["updated"] == nil || object["location"] == nil {
		t.Errorf("the update sent: %v", object)
	}

	// Taken off again: another Update, not taken for the first one again.
	places.of[noteID] = nil
	w.queue.sent = nil
	_ = w.h.HandleEvent(relocatedEvent(noteID, constants.NodeMediumNoteEntityType))
	if len(w.queue.sent) != 2 || w.queue.sent[0].payload["type"] != "Update" {
		t.Fatalf("after it was taken off: %+v", w.queue.sent)
	}
	object, _ = w.queue.sent[0].payload["object"].(map[string]any)
	if _, has := object["location"]; has {
		t.Errorf("still sent with a location: %v", object["location"])
	}

	// A topic's locations change: nothing goes.
	w.queue.sent = nil
	_ = w.h.HandleEvent(relocatedEvent(noteID, "node.medium.text-service.topic.v1"))
	if len(w.queue.sent) != 0 {
		t.Errorf("a topic went out: %+v", w.queue.sent)
	}
}
