package repositories

import (
	"strings"
	"testing"

	"anahita-libs/constants"
)

// The photos and videos a person is in: by an approved tag, whoever
// posted them, never one still waiting (roadmap row 103).
func TestATaggedListIsByApprovedTagNotByOwner(t *testing.T) {
	sql, args, err := actorFeedCondition(7, constants.FeedFilterTagged, nil, false).ToSql()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sql, "owner_id") {
		t.Errorf("by owner: %s", sql)
	}
	want := []any{constants.EdgeTagPersonType, int64(7), constants.TagApproved}
	if len(args) != len(want) {
		t.Fatalf("args %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("arg %d: %v, want %v", i, args[i], want[i])
		}
	}
}
