package handlers

import (
	"testing"

	an_entities "anahita-libs/data/entities"
	an_services "anahita-libs/data/services"

	"notification/data/services"
)

type fakeSocialgraph struct {
	an_services.SocialgraphGRPCInterface
	relation *an_entities.ActorRelation
}

func (f *fakeSocialgraph) Read(a, b int64) (*an_entities.ActorRelation, error) {
	return f.relation, nil
}

type fakeSubscriptions struct {
	an_services.SubscriptionsGRPCInterface
	deleted [][2]int64
}

func (f *fakeSubscriptions) Delete(source, target *an_entities.AnahitaNode) error {
	f.deleted = append(f.deleted, [2]int64{source.ID, target.ID})
	return nil
}

// WHAT THIS GUARDS (roadmap row 96, part 2): following a profile and
// administering it each subscribe; losing one keeps the subscription the
// other still makes.
func TestASubscriptionEndsWithTheLastReasonForIt(t *testing.T) {
	odo := &an_entities.AnahitaNode{ID: 2}
	promenade := &an_entities.AnahitaNode{ID: 10}

	for _, tc := range []struct {
		name     string
		relation an_entities.ActorRelation
		ends     bool
	}{
		{"still follows", an_entities.ActorRelation{IsLeading: true}, false},
		{"still administers", an_entities.ActorRelation{HasAdmin: true}, false},
		{"neither", an_entities.ActorRelation{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			subs := &fakeSubscriptions{}
			relation := tc.relation
			h := &eventHandler{Env: &Env{Srv: &services.Services{
				SocialgraphGRPC:   &fakeSocialgraph{relation: &relation},
				SubscriptionsGRPC: subs,
			}}}
			if err := h.unsubscribeUnlessWanted(odo, promenade); err != nil {
				t.Fatal(err)
			}
			if ended := len(subs.deleted) == 1; ended != tc.ends {
				t.Errorf("subscription ended: %v, want %v", ended, tc.ends)
			}
		})
	}
}
