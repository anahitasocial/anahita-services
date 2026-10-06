package services

import (
	"testing"

	"anahita-libs/data/entities"
)

func reply(id, parent int64, hidden bool) *entities.AnahitaNode {
	node := entities.NewAnahitaNode()
	node.ID = id
	node.ParentID = parent
	node.RootID = 1
	node.Hidden = hidden
	return node
}

func idsOf(replies []*entities.AnahitaNode) []int64 {
	var ids []int64
	for _, reply := range replies {
		ids = append(ids, reply.ID)
	}
	return ids
}

func equal(got []int64, want ...int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// A hidden reply takes everything said under it out of the thread with it.
func TestAHiddenReplyGoesWithEverythingUnderIt(t *testing.T) {
	thread := func() []*entities.AnahitaNode {
		return []*entities.AnahitaNode{
			reply(10, 1, false),
			reply(11, 1, true),   // hidden
			reply(12, 11, false), // under it
			reply(13, 12, false), // and under that
			reply(14, 10, false),
		}
	}

	if got := idsOf(VisibleInThread(thread(), false)); !equal(got, 10, 14) {
		t.Errorf("most people see %v, want 10 and 14", got)
	}

	// Whoever looks after the thread sees all of it, with the whole hidden
	// branch marked.
	all := VisibleInThread(thread(), true)
	if got := idsOf(all); !equal(got, 10, 11, 12, 13, 14) {
		t.Fatalf("whoever looks after the thread sees %v, want everything", got)
	}
	for _, r := range all {
		want := r.ID == 11 || r.ID == 12 || r.ID == 13
		if r.Hidden != want {
			t.Errorf("reply %d: marked hidden = %v, want %v", r.ID, r.Hidden, want)
		}
	}
}

func TestAThreadWithNothingHiddenIsUnchanged(t *testing.T) {
	thread := []*entities.AnahitaNode{reply(10, 1, false), reply(11, 10, false)}

	if got := idsOf(VisibleInThread(thread, false)); !equal(got, 10, 11) {
		t.Errorf("got %v", got)
	}
	if got := VisibleInThread(nil, false); len(got) != 0 {
		t.Errorf("an empty thread came back as %v", got)
	}
}
