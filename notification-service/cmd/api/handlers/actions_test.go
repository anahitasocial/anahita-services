package handlers

import (
	"testing"

	an_entities "anahita-libs/data/entities"
	an_services "anahita-libs/data/services"

	"notification/data/entities"
	"notification/data/services"
)

// fakeRelations answers from what it is given: who the viewer follows, and
// who is still waiting to follow whom.
type fakeRelations struct {
	an_services.SocialgraphGRPCInterface
	followed map[int64]bool    // the viewer follows them
	waiting  map[[2]int64]bool // [target, asker]
}

func (f *fakeRelations) Browse(ids []int64, viewerID int64) (map[int64]*an_entities.ActorRelation, error) {
	out := map[int64]*an_entities.ActorRelation{}
	for _, id := range ids {
		out[id] = &an_entities.ActorRelation{ActorAId: id, ActorBId: viewerID, IsLeading: f.followed[id]}
	}
	return out, nil
}

func (f *fakeRelations) Read(a, b int64) (*an_entities.ActorRelation, error) {
	return &an_entities.ActorRelation{ActorAId: a, ActorBId: b, HasRequested: f.waiting[[2]int64{a, b}]}, nil
}

func told(id int64, kind string, subject, target int64) *entities.Notification {
	n := entities.NewNotification()
	n.ID, n.Type = id, kind
	n.Subject.ID, n.Target.ID = subject, target
	return n
}

// WHAT THIS GUARDS (roadmap row 97, part 2): what a reader is offered is
// what is still to be done, as the graph has it now.
func TestActionsAreWhatIsStillToBeDone(t *testing.T) {
	viewer := an_entities.NewViewer()
	viewer.ID = 1
	h := &notificationHandler{Env: &Env{Srv: &services.Services{
		SocialgraphGRPC: &fakeRelations{
			followed: map[int64]bool{3: true},
			waiting:  map[[2]int64]bool{{1, 4}: true},
		},
	}}}

	items := []*entities.Notification{
		told(1, NotificationTypeActorFollow, 2, 1),        // Worf follows; not followed back
		told(2, NotificationTypeActorFollow, 3, 1),        // Dax follows; followed already
		told(3, NotificationTypeActorFollowRequest, 4, 1), // Odo asks; still waiting
		told(4, NotificationTypeActorFollowRequest, 5, 1), // Kira asked; answered
		told(5, NotificationTypeReplyAdd, 2, 1),
		told(6, NotificationTypeVoteup, 2, 1),
	}
	h.setActions(items, viewer)

	want := map[int64]string{1: ActionFollow, 2: "", 3: ActionAnswerRequest, 4: "", 5: ActionReply, 6: ""}
	for _, item := range items {
		got := ""
		if len(item.Actions) > 0 {
			got = item.Actions[0]
		}
		if got != want[item.ID] {
			t.Errorf("notification %d (%s): %q, want %q", item.ID, item.Type, got, want[item.ID])
		}
	}
}
