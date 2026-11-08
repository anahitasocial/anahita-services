package handlers

import (
	"testing"

	"anahita-libs/event"

	"notification/data/repositories"
	"notification/data/services"
)

// WHAT THIS GUARDS (roadmap row 124, part 2): a follower of somebody who
// moved is told, as followed or as asked, about the old profile and the new.
func TestAFollowerIsToldOfAMove(t *testing.T) {
	const amos, oldNaomi, newNaomi = 1, 100, 200
	kept := &keptNotifications{}
	h := &eventHandler{Env: &Env{
		Srv:   &services.Services{SocialgraphGRPC: &mutingRelations{}},
		Repos: &repositories.Repositories{Notifications: kept},
	}}

	for _, name := range []string{event.RemoteMovedFollowed, event.RemoteMovedAsked, "federation.moved.x.person.other.v1"} {
		if err := h.HandleEvents(&event.EventPayload{
			Name: name, SourceService: "activitypub-service",
			SubjectID: oldNaomi, SubjectType: "node.actor.activitypub-service.person.v1",
			ObjectID: newNaomi, ObjectType: "node.actor.activitypub-service.person.v1",
			TargetID: amos,
		}); err != nil {
			t.Fatal(err)
		}
	}

	if len(kept.made) != 2 {
		t.Fatalf("told %d times, want 2", len(kept.made))
	}
	for i, want := range []string{NotificationTypeActorMoved, NotificationTypeActorMovedAsked} {
		n := kept.made[i]
		if n.Type != want || n.RecipientID != amos || n.Subject.ID != oldNaomi || n.Target.ID != newNaomi {
			t.Errorf("%d: %+v", i, n)
		}
	}
}
