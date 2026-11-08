package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"

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

// muteEdgeLasting is the SQL of a lasting mute by the viewer, on the edge
// aliased `m`, whose muted person is matched by the caller.
const muteEdgeLasting = `m.type = ? AND m.node_a_id = ? AND (m.ends_at IS NULL OR m.ends_at > UTC_TIMESTAMP())`

// mutedCondition keeps the rows of a list that the viewer has not muted:
//
//   - a post or a reply, unless the viewer muted who wrote it or whose
//     profile it is on;
//   - a repost, unless the viewer muted the reposter, or who wrote the post
//     it repeats or whose profile that is on;
//   - a profile, always: a muted person is still found, to be unmuted.
//
// One way only, unlike a block: the muted person's lists are untouched.
func mutedCondition(viewerID int64) sq.Sqlizer {
	mine := sq.Expr(`NOT EXISTS (
          SELECT 1 FROM edges m
          WHERE `+muteEdgeLasting+`
            AND m.node_b_id IN (COALESCE(node.owner_id, 0), COALESCE(node.created_by, 0))
        )`, constants.EdgeMuteEntityType, viewerID)

	repeated := sq.Expr(`NOT EXISTS (
          SELECT 1 FROM nodes AS reposted
          JOIN edges m ON m.node_b_id IN (COALESCE(reposted.owner_id, 0), COALESCE(reposted.created_by, 0))
          WHERE reposted.id = node.parent_id
            AND `+muteEdgeLasting+`
        )`, constants.EdgeMuteEntityType, viewerID)

	return sq.Or{
		sq.Like{"node.type": constants.NodeActorPrefixEntityType + "%"},
		sq.And{
			mine,
			sq.Or{
				sq.NotEq{"node.type": constants.NodeBaseRepostType},
				repeated,
			},
		},
	}
}

// SetMutedQuery leaves out of a list what the viewer muted (mutedCondition).
// For the lists a person reads as theirs: the Following feed, custom feeds,
// a hashtag's and a location's posts, search. Not for a post opened by its
// address, a bookmark, or the muted person's own profile. The row is aliased
// `node`.
func SetMutedQuery(sb sq.SelectBuilder, viewer *entities.Viewer) sq.SelectBuilder {
	if viewer == nil || viewer.ID <= 0 {
		return sb
	}
	return sb.Where(mutedCondition(viewer.ID))
}

// SetMutedQueryOnProfile is SetMutedQuery for a profile's feed: nothing is
// left out of the feed of somebody the viewer muted, which they opened.
func SetMutedQueryOnProfile(sb sq.SelectBuilder, viewer *entities.Viewer, profileID int64) sq.SelectBuilder {
	if viewer == nil || viewer.ID <= 0 {
		return sb
	}
	return sb.Where(sq.Or{
		sq.Expr(`EXISTS (
          SELECT 1 FROM edges m
          WHERE `+muteEdgeLasting+`
            AND m.node_b_id = ?
        )`, constants.EdgeMuteEntityType, viewer.ID, profileID),
		mutedCondition(viewer.ID),
	})
}

// isTagEdges reports whether these edges lead from a hashtag or a location
// to its posts: a listing the viewer's mutes apply to.
func isTagEdges(edgeTypes []string) bool {
	if len(edgeTypes) == 0 {
		return false
	}
	for _, t := range edgeTypes {
		if t != constants.EdgeTagHashtagType && t != constants.EdgeTagLocationType {
			return false
		}
	}
	return true
}

// setMutedNeighboursQuery applies the viewer's mutes to a hashtag's or a
// location's posts, and to no other list of neighbours.
func setMutedNeighboursQuery(query sq.SelectBuilder, edgeTypes []string, viewer *entities.Viewer) sq.SelectBuilder {
	if !isTagEdges(edgeTypes) {
		return query
	}
	return SetMutedQuery(query, viewer)
}

// MutedAmong answers which of these profiles the viewer mutes, while the
// mute lasts: for a thread, whose replies the viewer would rather not see.
func (r *socialgraphRelRepo) MutedAmong(viewerID int64, actorIDs []int64) (map[int64]bool, error) {
	muted := map[int64]bool{}
	if viewerID <= 0 || len(actorIDs) == 0 {
		return muted, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query, args, err := sq.Select("e.node_b_id").
		From("edges e").
		Where(sq.Eq{
			"e.type":      constants.EdgeMuteEntityType,
			"e.node_a_id": viewerID,
			"e.node_b_id": actorIDs,
		}).
		Where(muteLasts("e")).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		muted[id] = true
	}
	return muted, rows.Err()
}

// WithoutMuted takes out of a thread, oldest first, the replies of people
// the viewer muted and everything under them. Nothing is taken from a thread
// under a post by somebody the viewer muted: they opened it.
func WithoutMuted(replies []*entities.AnahitaNode, muted map[int64]bool) []*entities.AnahitaNode {
	if len(muted) == 0 {
		return replies
	}
	gone := map[int64]bool{}
	kept := make([]*entities.AnahitaNode, 0, len(replies))
	for _, reply := range replies {
		if muted[reply.CreatedBy] || gone[reply.ParentID] {
			gone[reply.ID] = true
			continue
		}
		kept = append(kept, reply)
	}
	return kept
}

// deleteEndedMutes takes away mutes whose end has passed, a batch at a time.
const deleteEndedMutes = `
DELETE FROM edges
WHERE type = ?
  AND ends_at IS NOT NULL
  AND ends_at <= UTC_TIMESTAMP()
LIMIT ?`

// endedMutesBatch is the most mutes one run deletes; the rest wait for the
// next run, an hour on.
const endedMutesBatch = 5000

// DeleteEndedMutes deletes the mutes that have ended, for the sweeper. They
// stopped counting the moment they ended; this keeps the table from holding
// them.
func (r *socialgraphRelRepo) DeleteEndedMutes() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	result, err := r.db.ExecContext(ctx, deleteEndedMutes, constants.EdgeMuteEntityType, endedMutesBatch)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
