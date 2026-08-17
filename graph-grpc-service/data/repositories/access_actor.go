package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

func SetActorAccessQuery(sb sq.SelectBuilder, viewer *entities.Viewer) sq.SelectBuilder {
	if viewer.IsAdmin() {
		return sb
	}

	vID := viewer.ID
	follows := relViewerFollowsNode(vID)
	leaders := relNodeFollowsViewer(vID)
	mutual := sq.And{follows, leaders}
	isAdmin := relViewerAdminsNode(vID)
	notBlocked := relNotBlockedEitherWay(vID)
	selfOrAdmin := relSelfOrAdminForNode(vID)

	// Hard deny if blocked either way
	sb = sb.Where(notBlocked)

	// Build the access gate for the ACTOR node (profile visibility)
	nodeGate := sq.Or{
		sq.Expr("node.access IS NULL"),
		sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessPublic),
	}

	if vID != 0 {
		nodeGate = append(nodeGate,
			// registered
			sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessRegistered),

			// myself (person ⇒ self; non-person ⇒ admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessMyself),
				selfOrAdmin,
			},

			// followers (inclusive: follow/leader/mutual/admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessFollowers),
				sq.Or{follows, leaders, mutual, isAdmin},
			},

			// leaders (inclusive: leader/mutual/admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessLeaders),
				sq.Or{leaders, mutual, isAdmin},
			},

			// mutuals (inclusive: mutual/admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessMutuals),
				sq.Or{mutual, isAdmin},
			},

			// admins
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessAdmins),
				isAdmin,
			},
		)
	}

	// cases := sq.Case().When(
	// 	"node.access IS NULL",
	// 	"1",
	// ).When(
	// 	fmt.Sprintf("FIND_IN_SET('%s', node.access)", constants.AnahitaAccessPublic),
	// 	"1",
	// )

	// if viewer.ID > 0 {
	// 	// Viewer
	// 	cases = cases.When(
	// 		fmt.Sprintf("FIND_IN_SET('%s', node.access)", constants.AnahitaAccessRegistered),
	// 		"1",
	// 	).When(
	// 		fmt.Sprintf("node.id=%d", viewer.ID),
	// 		"1",
	// 	).When(
	// 		fmt.Sprintf("FIND_IN_SET('%d', node.access)", viewer.ID),
	// 		"1",
	// 	)

	// 	// Socialgraph
	// 	cases = cases.When(
	// 		fmt.Sprintf("FIND_IN_SET('%s', node.access)", constants.AnahitaAccessFollowers),
	// 		fmt.Sprintf("FIND_IN_SET('%d', node.follower_ids)", viewer.ID),
	// 	).When(
	// 		fmt.Sprintf("FIND_IN_SET('%s', node.access)", constants.AnahitaAccessLeaders),
	// 		fmt.Sprintf("FIND_IN_SET('%d', node.leader_ids)", viewer.ID),
	// 	).When(
	// 		fmt.Sprintf("FIND_IN_SET('%s', node.access)", constants.AnahitaAccessMutuals),
	// 		fmt.Sprintf("FIND_IN_SET('%d', node.mutual_ids)", viewer.ID),
	// 	).When(
	// 		fmt.Sprintf("FIND_IN_SET('%s', node.access)", constants.AnahitaAccessAdmins),
	// 		fmt.Sprintf("FIND_IN_SET('%d', node.administrator_ids)", viewer.ID),
	// 	).Else("0")
	// }

	// return sb.Where(cases)

	return sb.Where(nodeGate)
}

// relViewerFollowsNode checks if the viewer follows the node.
func relViewerFollowsNode(viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        EXISTS (
          SELECT 1 FROM edges e
          WHERE e.type = ?
            AND e.node_a_id = ?
            AND e.node_b_id = node.id
        )`,
		constants.EdgeFollowEntityType, viewerID,
	)
}

// relNodeFollowsViewer checks if the node follows the viewer.
func relNodeFollowsViewer(viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        EXISTS (
          SELECT 1 FROM edges e
          WHERE e.type = ?
            AND e.node_a_id = node.id
            AND e.node_b_id = ?
        )`,
		constants.EdgeFollowEntityType, viewerID,
	)
}

// relViewerAdminsNode checks if the viewer is an admin of the node.
func relViewerAdminsNode(viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        EXISTS (
          SELECT 1 FROM edges e
          WHERE e.type = ?
            AND e.node_a_id = ?
            AND e.node_b_id = node.id
        )`,
		constants.EdgeActorAdminEntityType, viewerID,
	)
}

// relNotBlockedEitherWay checks if the viewer is not blocked by the node.
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
// 		constants.EdgeBlockEntityType, viewerID, viewerID,
// 	)
// }

// relSelfOrAdminForNode checks if the viewer is the node itself or an admin of the node.
// If node is a person: "myself" means viewer == node.id.
// Else (group/event/...): "myself" means viewer is an admin of node.
func relSelfOrAdminForNode(viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        (
          (node.type LIKE 'node.actor.person.%' AND node.id = ?)
          OR
          (node.type NOT LIKE 'node.actor.person.%' AND EXISTS (
              SELECT 1 FROM edges e
              WHERE e.type = ?
                AND e.node_a_id = ?
                AND e.node_b_id = node.id
          ))
        )`,
		viewerID, constants.EdgeActorAdminEntityType, viewerID,
	)
}
