package responses

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"anahita-libs/data/entities"
)

func TestEventState(t *testing.T) {
	start := time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)

	cases := []struct {
		name      string
		now       time.Time
		cancelled bool
		want      string
	}{
		{"before it starts", start.Add(-time.Minute), false, EventStateUpcoming},
		{"the moment it starts", start, false, EventStateHappening},
		{"while it is on", start.Add(time.Hour), false, EventStateHappening},
		{"the moment it ends", end, false, EventStatePast},
		{"afterwards", end.Add(time.Hour), false, EventStatePast},
		{"called off, whenever", start.Add(-time.Hour), true, EventStateCancelled},
	}

	for _, c := range cases {
		if got := EventState(start, end, c.cancelled, c.now); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// Where to join online is sent only when the caller said the viewer may be
// shown it. That there is one is said either way.
func TestEventOnlineURLIsHeldBack(t *testing.T) {
	node := entities.NewAnahitaNode()
	node.StartsAt = time.Now().Add(time.Hour)
	node.EndsAt = time.Now().Add(2 * time.Hour)
	node.Event = &entities.EventDetails{
		OnlineURL:  "https://meet.example/SECRET",
		Capacity:   2,
		GoingCount: 2,
	}

	raw, _ := json.Marshal(NewEvent(node))
	if strings.Contains(string(raw), "SECRET") {
		t.Errorf("the address is sent to somebody not shown it: %s", raw)
	}
	if !strings.Contains(string(raw), `"has_online_url":true`) || !strings.Contains(string(raw), `"is_full":true`) {
		t.Errorf("missing has_online_url or is_full: %s", raw)
	}

	node.EventShowsOnlineURL = true
	raw, _ = json.Marshal(NewEvent(node))
	if !strings.Contains(string(raw), "SECRET") {
		t.Errorf("the address is not sent to somebody going: %s", raw)
	}

	if NewEvent(entities.NewAnahitaNode()) != nil {
		t.Error("a node that is not an event has an event block")
	}
}
