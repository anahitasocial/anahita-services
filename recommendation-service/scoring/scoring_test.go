package scoring

import (
	"fmt"
	"testing"
	"time"

	"recommendation/constants"
)

var now = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

func recently(ids ...int64) map[int64]time.Time {
	active := map[int64]time.Time{}
	for _, id := range ids {
		active[id] = now.Add(-24 * time.Hour)
	}
	return active
}

func order(ranked []Suggestion) string {
	ids := make([]int64, 0, len(ranked))
	for _, suggestion := range ranked {
		ids = append(ids, suggestion.ActorID)
	}
	return fmt.Sprint(ids)
}

func TestRankAddsUpWhatWasFoundAndSaysWhy(t *testing.T) {
	found := map[string][]Hit{
		constants.ReasonFollowedByLeaders: {{ID: 4, Count: 2, IDs: []int64{2, 3}}, {ID: 5, Count: 1, IDs: []int64{2}}},
		constants.ReasonFollowsYou:        {{ID: 5, Count: 1}},
		constants.ReasonPopular:           {{ID: 6, Count: 30}},
	}

	ranked := Rank(found, recently(4, 5, 6), now)

	// 5: follows you (4) + one leader (3) = 7. 4: two leaders = 6. 6: 0.5.
	if got := order(ranked); got != "[5 4 6]" {
		t.Fatalf("order %s", got)
	}
	if ranked[0].Score != 7 || ranked[1].Score != 6 || ranked[2].Score != 0.5 {
		t.Errorf("scores %v %v %v", ranked[0].Score, ranked[1].Score, ranked[2].Score)
	}

	// The reason that counted most leads.
	if ranked[0].Reasons[0].Type != constants.ReasonFollowsYou || ranked[0].Reasons[1].Type != constants.ReasonFollowedByLeaders {
		t.Errorf("reasons for 5: %+v", ranked[0].Reasons)
	}
	if got := fmt.Sprint(ranked[1].Reasons[0].IDs); got != "[2 3]" || ranked[1].Reasons[0].Count != 2 {
		t.Errorf("through whom 4 was found: %s, count %d", got, ranked[1].Reasons[0].Count)
	}
}

func TestRankCapsAReason(t *testing.T) {
	found := map[string][]Hit{
		constants.ReasonFollowedByLeaders: {{ID: 4, Count: 40, IDs: []int64{1, 2, 3, 4, 5}}},
		constants.ReasonPopular:           {{ID: 5, Count: 900}},
	}

	ranked := Rank(found, recently(4, 5), now)

	if ranked[0].Score != constants.Weights[constants.ReasonFollowedByLeaders].Most {
		t.Errorf("forty leaders scored %v", ranked[0].Score)
	}
	if got := len(ranked[0].Reasons[0].IDs); got != constants.MaxIDsInAReason {
		t.Errorf("%d ids named in a reason", got)
	}
	if ranked[0].Reasons[0].Count != 40 {
		t.Errorf("the count was cut with the ids: %d", ranked[0].Reasons[0].Count)
	}
	if ranked[1].Score != 0.5 {
		t.Errorf("popular scored %v", ranked[1].Score)
	}
}

func TestRankPutsTheQuietAndTheDormantLast(t *testing.T) {
	found := map[string][]Hit{
		constants.ReasonFollowsYou: {{ID: 1, Count: 1}, {ID: 2, Count: 1}, {ID: 3, Count: 1}, {ID: 4, Count: 1}},
	}
	active := map[int64]time.Time{
		1: now.Add(-10 * 24 * time.Hour),
		2: now.Add(-200 * 24 * time.Hour),
		3: now.Add(-500 * 24 * time.Hour),
		// 4 has never been seen.
	}

	ranked := Rank(found, active, now)

	// Alive, quiet, then the two that are dormant, by id.
	if got := order(ranked); got != "[1 2 3 4]" {
		t.Fatalf("order %s", got)
	}
	if ranked[0].Score != 4 || ranked[1].Score != 2 || ranked[2].Score != 0.4 || ranked[3].Score != 0.4 {
		t.Errorf("scores %v %v %v %v", ranked[0].Score, ranked[1].Score, ranked[2].Score, ranked[3].Score)
	}
}

func TestRankIsTheSameEachTime(t *testing.T) {
	found := map[string][]Hit{
		constants.ReasonFeatured: {{ID: 9, Count: 1}, {ID: 3, Count: 1}, {ID: 7, Count: 1}},
		constants.ReasonPopular:  {{ID: 0, Count: 5}, {ID: 8, Count: 0}},
	}

	for i := 0; i < 20; i++ {
		if got := order(Rank(found, recently(3, 7, 9, 8), now)); got != "[3 7 9]" {
			t.Fatalf("order %s", got)
		}
	}
}
