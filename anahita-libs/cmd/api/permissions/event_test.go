package permissions

import (
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

func eventNode(access string) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = 70
	n.Type = constants.NodeActorEventEntityType
	n.Access = access
	n.Enabled = true
	return n
}

func TestCanRsvp(t *testing.T) {
	now := time.Now()
	archived := eventNode("public")
	archived.ArchivedAt = &now

	cases := []struct {
		name   string
		viewer *entities.Viewer
		event  *entities.AnahitaNode
		rel    *entities.ActorRelation
		want   bool
	}{
		{"an event anybody can see", viewerAs(100), eventNode("public"), nil, true},
		{"one for its followers, by a stranger", viewerAs(100), eventNode("followers"), nil, false},
		{"one for its followers, by somebody already in", viewerAs(100), eventNode("followers"), &entities.ActorRelation{IsLeading: true}, true},
		// The invitation is the event's consent.
		{"one for its followers, by somebody invited", viewerAs(100), eventNode("followers"), &entities.ActorRelation{HasInvite: true}, true},
		{"somebody invited and since blocked", viewerAs(100), eventNode("public"), &entities.ActorRelation{HasInvite: true, IsBlocking: true}, false},
		{"an archived event", viewerAs(100), archived, nil, false},
		{"a guest", entities.NewViewer(), eventNode("public"), nil, false},
	}

	for _, c := range cases {
		if got := CanRsvp(c.viewer, c.event, c.rel); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCanSeeOnlineURL(t *testing.T) {
	going := &entities.EventDetails{ViewerRsvp: constants.EventRsvpGoing}
	maybe := &entities.EventDetails{ViewerRsvp: constants.EventRsvpMaybe}
	none := &entities.EventDetails{}

	cases := []struct {
		name    string
		viewer  *entities.Viewer
		rel     *entities.ActorRelation
		details *entities.EventDetails
		want    bool
	}{
		{"somebody going", viewerAs(100), nil, going, true},
		{"somebody who said maybe", viewerAs(100), nil, maybe, false},
		{"somebody invited who has not answered", viewerAs(100), &entities.ActorRelation{HasInvite: true}, none, false},
		{"whoever looks after it", viewerAs(100), &entities.ActorRelation{HasAdmin: true}, none, true},
		{"a site administrator", viewerWith(constants.PersonTypeAdmin), nil, none, true},
		{"a guest", entities.NewViewer(), nil, going, false},
		{"no details", viewerAs(100), nil, nil, false},
	}

	for _, c := range cases {
		if got := CanSeeOnlineURL(c.viewer, eventNode("public"), c.rel, c.details); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCanCancelEvent(t *testing.T) {
	if !CanCancelEvent(viewerAs(100), eventNode("public"), &entities.ActorRelation{HasAdmin: true}) {
		t.Error("whoever looks after an event cannot call it off")
	}
	if CanCancelEvent(viewerAs(100), eventNode("public"), &entities.ActorRelation{IsLeading: true}) {
		t.Error("somebody going can call it off")
	}
	if !CanCancelEvent(viewerWith(constants.PersonTypeAdmin), eventNode("public"), nil) {
		t.Error("a site administrator cannot call it off")
	}
	if CanCancelEvent(entities.NewViewer(), eventNode("public"), nil) {
		t.Error("a guest can call it off")
	}
}
