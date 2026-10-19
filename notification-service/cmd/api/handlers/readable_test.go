package handlers

import (
	"testing"

	an_entities "anahita-libs/data/entities"
	an_services "anahita-libs/data/services"

	"notification/data/entities"
	"notification/data/repositories"
	"notification/data/services"
)

// fakeGraph answers FilterVisible from a list, for every reader alike.
type fakeGraph struct {
	an_services.GraphNodeGRPCInterface
	visible map[int64]bool
	readers []int64
}

func (f *fakeGraph) FilterVisible(ids []int64, viewer *an_entities.Viewer) ([]int64, error) {
	f.readers = append(f.readers, viewer.ID)
	out := []int64{}
	for _, id := range ids {
		if f.visible[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

type fakeNotifications struct {
	repositories.NotificationInterface
	deleted []int64
}

func (f *fakeNotifications) Delete(n *entities.Notification) error {
	f.deleted = append(f.deleted, n.ID)
	return nil
}

func notification(id, object, reply int64) *entities.Notification {
	n := entities.NewNotification()
	n.ID = id
	n.Object.ID = object
	n.CommentID = reply
	return n
}

// WHAT THIS GUARDS (roadmap row 96, part 1): the list shows only what its
// reader may still read, and a notification about anything else goes, so
// that the count agrees with the list.
func TestTheListShowsOnlyWhatItsReaderMayRead(t *testing.T) {
	graph := &fakeGraph{visible: map[int64]bool{100: true, 300: true}}
	repo := &fakeNotifications{}
	h := &notificationHandler{Env: &Env{
		Srv:   &services.Services{GraphNodeGRPC: graph},
		Repos: &repositories.Repositories{Notifications: repo},
	}}

	kept, gone := h.keepReadable([]*entities.Notification{
		notification(1, 100, 0),   // a like of a post they may read
		notification(2, 200, 0),   // a post made private since
		notification(3, 100, 300), // a reply they may read, in a thread they may
		notification(4, 100, 400), // a reply they may not
		notification(5, 0, 0),     // a follow: no post at all
	}, an_entities.NewViewer())

	got := []int64{}
	for _, n := range kept {
		got = append(got, n.ID)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Errorf("kept %v, want [1 3 5]", got)
	}
	if gone != 2 || len(repo.deleted) != 2 || repo.deleted[0] != 2 || repo.deleted[1] != 4 {
		t.Errorf("deleted %v (%d), want [2 4]", repo.deleted, gone)
	}
	if len(graph.readers) != 1 {
		t.Errorf("asked the graph %d times, want once", len(graph.readers))
	}
}

// Nobody is told of a post they may not see, asked as who they are.
func TestNobodyIsToldOfWhatTheyMayNotRead(t *testing.T) {
	graph := &fakeGraph{visible: map[int64]bool{100: true}}
	h := &eventHandler{Env: &Env{Srv: &services.Services{GraphNodeGRPC: graph}}}

	if !h.mayRead(7, 100) {
		t.Error("not told of a post they may read")
	}
	if h.mayRead(7, 100, 400) {
		t.Error("told of a reply they may not read")
	}
	if h.mayRead(7, 200) {
		t.Error("told of a post they may not read")
	}
	for _, reader := range graph.readers {
		if reader != 7 {
			t.Errorf("asked as %d, want the recipient, 7", reader)
		}
	}
}
