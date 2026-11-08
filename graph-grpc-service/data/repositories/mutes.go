package repositories

import (
	"anahita-libs/constants"

	sq "github.com/Masterminds/squirrel"
)

// Mutes (roadmap row 133). A mute is an edge from who muted to who is muted
// (constants.EdgeMuteEntityType). Unlike a block it is not part of the
// access gate: it does not say what somebody may see, only what they would
// rather not have in their lists. A post opened by its address, a bookmark
// and the muted person's own profile are untouched by it.

// muteLasts is whether the mute on the edge aliased `alias` has not ended. A
// mute is over the moment its ends_at passes; the sweeper deletes the row
// later, and nothing waits for it.
func muteLasts(alias string) sq.Sqlizer {
	return sq.Expr("(" + alias + ".ends_at IS NULL OR " + alias + ".ends_at > UTC_TIMESTAMP())")
}

// setLastingMutesQuery keeps, in a list of whom somebody muted, the mutes
// that have not ended.
func setLastingMutesQuery(query sq.SelectBuilder, edgeTypes []string) sq.SelectBuilder {
	if len(edgeTypes) != 1 || edgeTypes[0] != constants.EdgeMuteEntityType {
		return query
	}
	return query.Where(muteLasts("edge"))
}
