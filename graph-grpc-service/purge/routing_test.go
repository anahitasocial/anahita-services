package purge

import (
	"strings"
	"testing"

	"anahita-libs/data/entities"
)

// The consumer binds to "node.actor.*.*.*.v1", where AMQP's * matches exactly
// one word. A key with the wrong number of segments matches nothing and the
// message is silently dropped — which is how the purge event went undelivered
// without anything failing.
func TestPurgedRoutingKeyMatchesBinding(t *testing.T) {
	cases := []struct {
		name    string
		service string
		typ     string
		want    string
	}{
		{"group", "group-service", "node.actor.group-service.group.v1", "node.actor.group.group.purged.v1"},
		{"person", "person-service", "node.actor.person-service.person.v1", "node.actor.person.person.purged.v1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := entities.NewAnahitaNode()
			node.Service = tc.service
			node.Type = tc.typ

			got := purgedRoutingKey(node)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}

			// Six segments, matching node.actor.*.*.*.v1.
			if n := len(strings.Split(got, ".")); n != 6 {
				t.Errorf("got %d segments, binding requires 6: %q", n, got)
			}

			// And the consumer reads the action from index 4.
			if action := strings.Split(got, ".")[4]; action != "purged" {
				t.Errorf("consumer would read action %q, want \"purged\"", action)
			}
		})
	}
}

// A malformed type must not panic the sweep. Splitting bare on [3], as the
// other publishers do, would take down a whole batch for one bad row.
func TestPurgedRoutingKeySurvivesMalformedNode(t *testing.T) {
	node := entities.NewAnahitaNode()
	node.Service = ""
	node.Type = "nonsense"

	if got := purgedRoutingKey(node); got == "" {
		t.Error("expected a key, got empty")
	}
}
