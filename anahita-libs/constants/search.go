package constants

const (
	SearchScopeActor  = "actors"
	SearchScopeMedium = "media"
	SearchScopeAll    = "all"
	// Custom feeds people have shared. Among "all" as well.
	SearchScopeFeed = "feeds"
)

// What somebody may adjust of search's Most relevant for themselves, and to
// what (the user, 2026-10-16: as they adjust their recommendations). Each
// is a step that multiplies the installation's own weight: Off, Less, As
// is, More, Much more. The graph applies them (ranking.Weights.Adjusted);
// search-service takes and gives them.
var SearchAdjustable = []string{"like", "save", "reply", "follow", "cite", "following"}

// SearchSteps is what each may be set to. 1 is as the installation has it.
var SearchSteps = []float64{0, 0.5, 1, 1.5, 2}
