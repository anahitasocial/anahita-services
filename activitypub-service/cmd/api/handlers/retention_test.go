package handlers

import (
	"testing"
)

// THE RULE THIS GUARDS: what a rule says is to go, goes as it would have
// had its own server said so: the post, and then whoever nothing is left
// of, with what this service keeps of them.
func TestWhatIsDueToGoIsRemovedAsIfTakenAwayThere(t *testing.T) {
	w, alice := settledWorld()
	w.fromAlice(received(alicesNote, "<p>hi</p>", ""))
	note, _ := w.nodes.find(func(n *entitiesNode) bool { return n.URI == alicesNote })
	if note == nil {
		t.Fatal("her note was not kept to begin with")
	}

	// A post that is not there any more is passed over.
	posts, people := w.h.Tidy([]int64{note.ID, 99999}, nil)
	if posts != 1 || people != 0 {
		t.Fatalf("removed %d posts and %d people", posts, people)
	}
	if found, _ := w.nodes.find(func(n *entitiesNode) bool { return n.URI == alicesNote }); found != nil {
		t.Errorf("still kept: %+v", found)
	}
	if found, _ := w.nodes.find(func(n *entitiesNode) bool { return n.URI == aliceID }); found == nil {
		t.Fatal("she went with her post")
	}

	if _, people := w.h.Tidy(nil, []int64{alice.NodeID, 99999}); people != 1 {
		t.Fatalf("removed %d people", people)
	}
	if found, _ := w.nodes.find(func(n *entitiesNode) bool { return n.URI == aliceID }); found != nil || w.actors.kept[alice.NodeID] != nil {
		t.Errorf("still known: %+v", found)
	}
}
