package handlers

import (
	"context"
	"net/http"
	"testing"
	"time"

	"activitypub/data/repositories"

	"anahita-libs/data/entities"
)

// fakeMissed finds what it is told to find, every time it is asked: a
// sweep that puts something right must not put it right again.
type fakeMissed struct {
	notAsked, gone       []repositories.Pair
	notesNotSent, people []int64
	notesGone            []int64
}

func (f *fakeMissed) FollowsNotAsked(time.Time, int) ([]repositories.Pair, error) {
	return f.notAsked, nil
}
func (f *fakeMissed) FollowsGone(int) ([]repositories.Pair, error) { return f.gone, nil }
func (f *fakeMissed) NotesNotSent(time.Time, time.Time, int) ([]int64, error) {
	return f.notesNotSent, nil
}
func (f *fakeMissed) NotesGone(int) ([]int64, error) { return f.notesGone, nil }
func (f *fakeMissed) People(int) ([]int64, error)    { return f.people, nil }

func (w *inboxWorld) person(id int64) *entities.AnahitaNode {
	for _, node := range w.nodes.people {
		if node.ID == id {
			return node
		}
	}
	return nil
}

func (w *inboxWorld) sweep() Swept {
	return w.h.Sweep(context.Background(), time.Now().UTC())
}

// THE RULE THIS GUARDS: what the bus lost is put right once, by the same
// hands that would have handled the event, however often it is found.
func TestASweepSendsThePostTheBusLost(t *testing.T) {
	w := notesWorld()
	w.h.Missed = &fakeMissed{notesNotSent: []int64{noteID}}

	// Nobody heard of Ana's note. The sweep finds it.
	if swept := w.sweep(); swept.Notes != 1 || len(w.queue.sent) != 2 {
		t.Fatalf("swept %+v, queued %d, want one for each server", swept, len(w.queue.sent))
	}
	if w.queue.sent[0].payload["type"] != "Create" {
		t.Errorf("sent %v", w.queue.sent[0].payload["type"])
	}

	// Found again, by this copy or another, it is not sent again.
	w.sweep()
	if len(w.queue.sent) != 2 {
		t.Errorf("a second sweep queued %d more", len(w.queue.sent)-2)
	}
}

func TestASweepTakesBackThePostThatIsGone(t *testing.T) {
	w, _ := settledWorld()
	w.h.Missed = &fakeMissed{notesGone: []int64{noteID}}

	// Ana's note was sent, and has since been closed to her followers
	// only, with nobody told.
	w.note().Access = "followers"
	w.sweep()
	if len(w.queue.sent) != 2 || w.queue.sent[0].payload["type"] != "Delete" {
		t.Fatalf("queued %d: %+v", len(w.queue.sent), w.queue.sent)
	}
	if kept, _ := w.objects.Read(noteID); kept != nil {
		t.Error("the note is still on record as sent")
	}

	w.sweep()
	if len(w.queue.sent) != 2 {
		t.Errorf("a second sweep queued %d more", len(w.queue.sent)-2)
	}
}

func TestASweepSettlesTheFollowsTheBusLost(t *testing.T) {
	w := notesWorld()
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	var alice *repositories.RemoteActor
	for _, kept := range w.actors.kept {
		alice = kept
	}
	pair := []repositories.Pair{{LocalID: anaID, RemoteID: alice.NodeID}}

	// Ana follows Alice in the graph, and her server was never asked.
	w.h.Missed = &fakeMissed{notAsked: pair}
	w.queue.sent = nil
	if swept := w.sweep(); swept.Follows != 1 || len(w.queue.sent) != 1 || w.queue.sent[0].payload["type"] != "Follow" {
		t.Fatalf("swept %+v, queued %+v", swept, w.queue.sent)
	}
	asked, _ := w.follows.Read(anaID, alice.NodeID, repositories.FollowOut)
	if asked == nil || asked.State != repositories.FollowPending {
		t.Fatalf("the follow on record: %+v", asked)
	}
	w.sweep()
	if len(w.queue.sent) != 1 {
		t.Errorf("a second sweep asked again")
	}

	// She stops, and her server was never told.
	w.h.Missed = &fakeMissed{gone: pair}
	w.queue.sent = nil
	w.sweep()
	if len(w.queue.sent) != 1 || w.queue.sent[0].payload["type"] != "Undo" {
		t.Fatalf("queued %+v", w.queue.sent)
	}
	if left, _ := w.follows.Read(anaID, alice.NodeID, repositories.FollowOut); left != nil {
		t.Error("the follow is still on record")
	}
	w.sweep()
	if len(w.queue.sent) != 1 {
		t.Errorf("a second sweep told her server again")
	}
}

func TestASweepHasSomebodyLeaveWhoMayNoLongerBeThere(t *testing.T) {
	w, _ := settledWorld()
	w.h.Missed = &fakeMissed{people: []int64{anaID}}

	// Still open to everybody: nothing to do.
	if swept := w.sweep(); swept.Left != 0 || len(w.queue.sent) != 0 {
		t.Fatalf("swept %+v, queued %d", swept, len(w.queue.sent))
	}

	// Her profile was closed, and the word of it was lost.
	w.person(anaID).Access = "followers"
	if swept := w.sweep(); swept.Left != 1 {
		t.Fatalf("swept %+v", swept)
	}
	if account, _ := w.accounts.Read(anaID); account.Enabled {
		t.Error("she is still on the fediverse")
	}
	told := false
	for _, sent := range w.queue.sent {
		if sent.payload["type"] == "Delete" && sent.payload["object"] == anaActor {
			told = true
		}
	}
	if !told {
		t.Errorf("other servers were not told to forget her: %+v", w.queue.sent)
	}
}
