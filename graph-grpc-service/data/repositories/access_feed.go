package repositories

import (
	an_const "anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// SetFeedAccessQuery gates a feed. A feed mixes three kinds of row, and each
// is judged by the rule the rest of the package already uses for it:
//
//   - a post, by the medium gate: its owner's access and its own;
//   - a comment, by the post it sits under;
//   - a repost, by the post it repeats, and by the reposter's profile.
//
// This used to be a second, hand-written copy of the medium gate that read the
// node's access from `object`, the row joined through parent_id. Two things
// followed from that:
//
//   - A post has no parent, so `object.access IS NULL` was true and its own
//     access was never read. Every followers-only, mutuals-only and only-me
//     post showed in profile and home feeds, to guests included.
//   - A repost was judged by the original's access, but "is the viewer a
//     follower" was asked of the REPOSTER, the row's owner. Anybody following
//     the reposter saw a followers-only post of somebody they did not follow.
//
// The query must alias the row `node` and its owner `owner`.
func SetFeedAccessQuery(sb sq.SelectBuilder, viewer *entities.Viewer) sq.SelectBuilder {
	if viewer.IsAdmin() {
		return sb
	}

	derived := []string{an_const.NodeBaseCommentEntityType, an_const.NodeBaseRepostType}

	return sb.Where(sq.Or{
		sq.And{
			sq.NotEq{"node.type": derived},
			postAccessCondition(viewer),
		},
		sq.And{
			sq.Eq{"node.type": an_const.NodeBaseCommentEntityType},
			commentAccessCondition(viewer),
		},
		sq.And{
			sq.Eq{"node.type": an_const.NodeBaseRepostType},
			repostAccessCondition(viewer),
		},
	})
}

// repostAccessCondition admits a repost when the post it repeats passes the
// medium gate for this viewer, the reposter's profile admits them, and nobody
// on either side has blocked the other.
//
// The repost's own access column is not read. It is a copy of the original's,
// taken when the repost was made, and the original is the one that can change.
func repostAccessCondition(viewer *entities.Viewer) sq.Sqlizer {
	return sq.And{
		relNotBlockedEitherWay(viewer.ID, contentActorRefs("node")...),
		accessGate("owner.access", viewer),
		parentPassesMediumGate(viewer),
	}
}

// queryGetLeaderIds retrieves the IDs of the leaders that the viewer follows.
// func queryGetLeaderIds(viewerID int64) sq.Sqlizer {
// 	return sq.Expr(`
//         SELECT e.node_b_id FROM edges e
//         WHERE e.type = ?
//           AND e.node_a_id = ?
//     `, an_const.EdgeFollowEntityType, viewerID)
// }

// relViewerAdminsOwner checks if the viewer is an admin of the owner.
// func relViewerAdminsOwner(viewerID int64) sq.Sqlizer {
// 	return sq.Expr(`
//         EXISTS (
//           SELECT 1 FROM edges e
//           WHERE e.type = ?
//             AND e.node_a_id = ?
//             AND e.node_b_id = owner.id
//         )`,
// 		an_const.EdgeActorAdminEntityType, viewerID,
// 	)
// }

// relViewerFollowsOwner checks if the viewer follows the owner.
// func relViewerFollowsOwner(viewerID int64) sq.Sqlizer {
// 	return sq.Expr(`
//         EXISTS (
//           SELECT 1 FROM edges e
//           WHERE e.type = ?
//             AND e.node_a_id = ?
//             AND e.node_b_id = owner.id
//         )`,
// 		an_const.EdgeFollowEntityType, viewerID,
// 	)
// }

// relOwnerFollowsViewer checks if the owner follows the viewer.
// func relOwnerFollowsViewer(viewerID int64) sq.Sqlizer {
// 	return sq.Expr(`
//         EXISTS (
//           SELECT 1 FROM edges e
//           WHERE e.type = ?
//             AND e.node_a_id = owner.id
//             AND e.node_b_id = ?
//         )`,
// 		an_const.EdgeFollowEntityType, viewerID,
// 	)
// }

// relNotBlockedEitherWay checks if the viewer is not blocked by the owner.
// func relNotBlockedEitherWay(viewerID int64) sq.Sqlizer {
// 	return sq.Expr(`
//         NOT EXISTS (
//           SELECT 1 FROM edges e
//           WHERE e.type = ?
//             AND (
//                (e.node_a_id = ? AND e.node_b_id = node.id)
//             OR (e.node_b_id = ? AND e.node_a_id = node.id)
//             )
//         )`,
// 		an_const.EdgeBlockEntityType, viewerID, viewerID,
// 	)
// }

// relSelfOrAdminForOwner checks if the viewer is the owner itself or an admin of the owner.
// If owner is a person: myself = (viewer is the owner)
// Else (group/event/...): myself = (viewer admins the owner)
// func relSelfOrAdminForOwner(viewerID int64) sq.Sqlizer {
// 	return sq.Expr(`
//         (
//           (owner.type LIKE 'node.actor.person.%' AND owner.id = ?)
//           OR
//           (owner.type NOT LIKE 'node.actor.person.%' AND EXISTS (
//               SELECT 1 FROM edges e
//               WHERE e.type = ?
//                 AND e.node_a_id = ?
//                 AND e.node_b_id = owner.id
//           ))
//         )`,
// 		viewerID, an_const.EdgeActorAdminEntityType, viewerID,
// 	)
// }
