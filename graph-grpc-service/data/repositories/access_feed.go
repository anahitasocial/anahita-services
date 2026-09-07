package repositories

import (
	an_const "anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// SetFeedAccessQuery returns the access query for media repository
func SetFeedAccessQuery(sb sq.SelectBuilder, viewer *entities.Viewer) sq.SelectBuilder {
	if viewer.IsAdmin() {
		return sb
	}

	vID := viewer.ID
	follows := relViewerFollowsOwner(vID)
	leaders := relOwnerFollowsViewer(vID)
	mutual := sq.And{follows, leaders}
	isAdmin := relViewerAdminsOwner(vID)
	notBlocked := relNotBlockedEitherWay(vID, contentActorRefs("node")...)
	relSelfOrAdminForOwner := relSelfOrAdminForOwner(vID)

	// Hard deny if blocked either way
	sb = sb.Where(notBlocked)

	// --- OWNER gate ---
	ownerGate := sq.Or{
		sq.Expr("owner.access IS NULL"),
		sq.Expr("FIND_IN_SET(?, owner.access)", an_const.AnahitaAccessPublic),
	}

	if vID != 0 {
		ownerGate = append(ownerGate,
			// registered
			sq.Expr("FIND_IN_SET(?, owner.access)", an_const.AnahitaAccessRegistered),

			// myself (person ⇒ self, non-person ⇒ admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", an_const.AnahitaAccessMyself),
				relSelfOrAdminForOwner,
			},

			// followers (inclusive: follow/leader/mutual/admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", an_const.AnahitaAccessFollowers),
				sq.Or{follows, leaders, mutual, isAdmin},
			},

			// leaders (inclusive: leader/mutual/admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", an_const.AnahitaAccessLeaders),
				sq.Or{leaders, mutual, isAdmin},
			},

			// mutuals (inclusive: mutual/admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", an_const.AnahitaAccessMutuals),
				sq.Or{mutual, isAdmin},
			},

			// admins
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", an_const.AnahitaAccessAdmins),
				isAdmin,
			},
		)
	}

	nodeGate := sq.Or{
		sq.Expr("object.access IS NULL"),
		sq.Expr("FIND_IN_SET(?, object.access)", an_const.AnahitaAccessPublic),
	}

	if vID != 0 {
		nodeGate = append(nodeGate,
			// registered
			sq.Expr("FIND_IN_SET(?, object.access)", an_const.AnahitaAccessRegistered),

			// myself (same rule as owner gate)
			sq.And{
				sq.Expr("FIND_IN_SET(?, object.access)", an_const.AnahitaAccessMyself),
				relSelfOrAdminForOwner,
			},

			// followers (inclusive)
			sq.And{
				sq.Expr("FIND_IN_SET(?, object.access)", an_const.AnahitaAccessFollowers),
				sq.Or{follows, leaders, mutual, isAdmin},
			},

			// leaders (inclusive)
			sq.And{
				sq.Expr("FIND_IN_SET(?, object.access)", an_const.AnahitaAccessLeaders),
				sq.Or{leaders, mutual, isAdmin},
			},

			// mutuals (inclusive)
			sq.And{
				sq.Expr("FIND_IN_SET(?, object.access)", an_const.AnahitaAccessMutuals),
				sq.Or{mutual, isAdmin},
			},

			// admins
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", an_const.AnahitaAccessAdmins),
				isAdmin,
			},
		)
	}

	return sb.Where(ownerGate).Where(nodeGate)
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
