package requests

import "testing"

// The featured list is ordered by when each actor was featured, and the only
// way the graph applies that is by being asked for no other ordering. The
// updated_at default is set before binding, so it has to be dropped here —
// but only when the caller did not choose it.
func TestGraphOrderingLeavesTheFeaturedListToItsOwnOrder(t *testing.T) {
	for _, tc := range []struct {
		name          string
		featured      bool
		orderingGiven bool
		want          string
	}{
		{"featured, no ordering asked for", true, false, ""},
		{"featured, ordering asked for", true, true, "created_at"},
		{"not featured, default ordering", false, false, "created_at"},
		{"not featured, ordering asked for", false, true, "created_at"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &ActorBrowseRequest{Ordering: "created_at", Featured: tc.featured}

			if got := req.GraphOrdering(tc.orderingGiven); got != tc.want {
				t.Errorf("GraphOrdering(%v) = %q, want %q", tc.orderingGiven, got, tc.want)
			}
		})
	}
}
