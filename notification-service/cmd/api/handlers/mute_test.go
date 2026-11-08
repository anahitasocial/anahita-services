package handlers

import (
	"testing"

	an_entities "anahita-libs/data/entities"
	an_services "anahita-libs/data/services"

	"notification/data/entities"
	"notification/data/repositories"
	"notification/data/services"
)

type mutingRelations struct {
	an_services.SocialgraphGRPCInterface
	rel map[[2]int64]*an_entities.ActorRelation
}

func (f *mutingRelations) Read(a, b int64) (*an_entities.ActorRelation, error) {
	if rel, ok := f.rel[[2]int64{a, b}]; ok {
		return rel, nil
	}
	return &an_entities.ActorRelation{ActorAId: a, ActorBId: b}, nil
}

type keptNotifications struct {
	repositories.NotificationInterface
	made []*entities.Notification
}

func (f *keptNotifications) Add(n *entities.Notification) (int64, error) {
	f.made = append(f.made, n)
	return int64(len(f.made)), nil
}

// WHAT THIS GUARDS (roadmap row 133): somebody who muted another and hid
// their notifications is not told of what they do; with the notifications
// kept, or of anybody else, they are.
func TestAMuteWithItsNotificationsHiddenTellsNothing(t *testing.T) {
	const kira, odo, quark, nog = 1, 2, 3, 4
	kept := &keptNotifications{}
	h := &eventHandler{Env: &Env{
		Srv: &services.Services{SocialgraphGRPC: &mutingRelations{rel: map[[2]int64]*an_entities.ActorRelation{
			{kira, odo}:   {IsMuting: true, MuteHidesNotifications: true},
			{kira, quark}: {IsMuting: true, MuteHidesNotifications: false},
		}}},
		Repos: &repositories.Repositories{Notifications: kept},
	}}

	for _, by := range []int64{odo, quark, nog} {
		n := entities.NewNotification()
		n.RecipientID = kira
		n.Subject = &an_entities.AnahitaNode{ID: by}
		if _, err := h.add(n); err != nil {
			t.Fatal(err)
		}
	}

	var from []int64
	for _, n := range kept.made {
		from = append(from, n.Subject.ID)
	}
	if len(from) != 2 || from[0] != quark || from[1] != nog {
		t.Errorf("told of %v, want Quark's and Nog's", from)
	}
}
