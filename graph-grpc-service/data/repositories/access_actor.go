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

	return sb.Where(actorAccessCondition(viewer))
}

// actorAccessCondition is the gate for a profile: nobody blocked either way,
// and the actor's own access admits the viewer. The row it judges is aliased
// `node`.
//
// A condition rather than a builder step, so FilterVisible can apply it to
// the actors in a list that also holds posts and tags.
func actorAccessCondition(viewer *entities.Viewer) sq.Sqlizer {
	// Hard deny if blocked either way.
	return sq.And{
		relNotBlockedEitherWay(viewer.ID, "node.id"),
		actorAccessGate(viewer),
	}
}

// SetActorAccessOrAskableQuery is the gate for a LIST of actors that also
// shows the ones the viewer may not see but may ask to follow.
//
// Whoever uses it has to tell the two apart afterwards (FilterVisible does)
// and send the second kind as their name and picture only. It is for the
// lists of people and of groups, and nothing else: every other listing keeps
// SetActorAccessQuery.
func SetActorAccessOrAskableQuery(sb sq.SelectBuilder, viewer *entities.Viewer) sq.SelectBuilder {
	if viewer.IsAdmin() {
		return sb
	}

	// Asking is for somebody signed in, so a visitor's list is unchanged.
	if viewer.ID == 0 {
		return sb.Where(actorAccessCondition(viewer))
	}

	return sb.Where(sq.And{
		relNotBlockedEitherWay(viewer.ID, "node.id"),
		sq.Or{
			actorAccessGate(viewer),
			sq.Expr("node.allow_follow_request = 1"),
		},
	})
}

// actorAccessGate is whether the actor's own access admits the viewer,
// leaving blocks aside.
func actorAccessGate(viewer *entities.Viewer) sq.Sqlizer {
	vID := viewer.ID
	follows := relViewerFollowsNode(vID)
	leaders := relNodeFollowsViewer(vID)
	mutual := sq.And{follows, leaders}
	isAdmin := relViewerAdminsNode(vID)
	selfOrAdmin := relSelfOrAdminForNode(vID)

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

	return nodeGate
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
          (node.type LIKE 'node.actor.person-service.person.%' AND node.id = ?)
          OR
          (node.type NOT LIKE 'node.actor.person-service.person.%' AND EXISTS (
              SELECT 1 FROM edges e
              WHERE e.type = ?
                AND e.node_a_id = ?
                AND e.node_b_id = node.id
          ))
        )`,
		viewerID, constants.EdgeActorAdminEntityType, viewerID,
	)
}
