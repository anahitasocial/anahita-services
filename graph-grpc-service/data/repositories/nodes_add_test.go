package repositories

import (
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// A new node keeps the date it says it was written, as a channel's article
// keeps its item's, unless that date was never set or is still to come.
func TestANewNodeKeepsTheDateItSays(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	written := now.Add(-3 * time.Hour)

	for _, c := range []struct {
		name string
		said time.Time
		want time.Time
	}{
		{"its own date", written, written},
		{"never set", time.Time{}, now},
		{"set to 1970", time.Unix(0, 0), now},
		{"in the future", now.Add(time.Minute), now},
	} {
		if got := addedAt(c.said, now); !got.Equal(c.want) {
			t.Errorf("%s: made at %v, want %v", c.name, got, c.want)
		}
	}
}

// A person who signs up is asked before search engines may index them; a
// group, an event or a channel is made to be found (roadmap row 174).
func TestSearchEnginesAtFirst(t *testing.T) {
	for nodeType, want := range map[string]bool{
		constants.NodeActorPersonEntityType:  false,
		constants.NodeActorEventEntityType:   true,
		constants.NodeActorChannelEntityType: true,
		"node.actor.group-service.group.v1":  true,
	} {
		if got := searchEnginesAtFirst(&entities.AnahitaNode{Type: nodeType}); got != want {
			t.Errorf("%s starts with search engines %v, want %v", nodeType, got, want)
		}
	}
}
