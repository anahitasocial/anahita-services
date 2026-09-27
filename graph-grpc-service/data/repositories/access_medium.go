package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// SetMediumAccessQuery returns the access query for media repository.
//
// Comments are judged by the post they sit under, not by themselves. A comment
// carries no audience of its own — every one is stored with access ”, and
// none has an owner — so gated like a post, every comment failed the node gate
// and nobody but site administrators could see any. A comment is visible to
// whoever can see its post, as permissions.CommentPermissions.CanRead says.
func SetMediumAccessQuery(sb sq.SelectBuilder, viewer *entities.Viewer) sq.SelectBuilder {
	if viewer.IsAdmin() {
		return sb
	}

	return sb.Where(sq.Or{
		sq.And{
			sq.NotEq{"node.type": constants.NodeBaseCommentEntityType},
			mediumAccessCondition(viewer),
		},
		sq.And{
			sq.Eq{"node.type": constants.NodeBaseCommentEntityType},
			commentAccessCondition(viewer),
		},
	})
}

// commentAccessCondition admits a comment when its post passes the medium
// gate, and nobody on either side has blocked the other.
//
// The post is gated in a subquery that aliases it `node` and its owner
// `owner`, the names every medium gate helper is written against, so the gate
// is the medium one exactly rather than a copy of it rewritten for other
// aliases — which is how the gates in this package have drifted before. The
// inner aliases shadow the outer ones inside the subquery only.
func commentAccessCondition(viewer *entities.Viewer) sq.Sqlizer {
	postGate, args, err := sq.Select("node.id").
		From("nodes AS node").
		LeftJoin("nodes AS owner ON node.owner_id = owner.id").
		Where("node.deleted_at IS NULL").
		Where(mediumAccessCondition(viewer)).
		ToSql()
	if err != nil {
		// Fail closed: a gate that cannot be built admits nothing.
		return sq.Expr("1 = 0")
	}

	return sq.And{
		relNotBlockedEitherWay(viewer.ID, contentActorRefs("node")...),
		sq.Expr("node.parent_id IN ("+postGate+")", args...),
	}
}

// mediumAccessCondition is the gate for a post: nobody blocked either way,
// the owner's access admits the viewer, and so does the post's own.
func mediumAccessCondition(viewer *entities.Viewer) sq.Sqlizer {
	vID := viewer.ID
	follows := relViewerFollowsOwner(vID)
	leaders := relOwnerFollowsViewer(vID)
	mutual := sq.And{follows, leaders}
	isAdmin := relViewerAdminsOwner(vID)
	notBlocked := relNotBlockedEitherWay(vID, contentActorRefs("node")...)
	relSelfOrAdminForOwner := relSelfOrAdminForOwner(vID)

	// --- OWNER gate ---
	ownerGate := sq.Or{
		sq.Expr("owner.access IS NULL"),
		sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessPublic),
	}

	if vID != 0 {
		ownerGate = append(ownerGate,
			// registered
			sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessRegistered),

			// myself (person ⇒ self, non-person ⇒ admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessMyself),
				relSelfOrAdminForOwner,
			},

			// followers (inclusive: follow/leader/mutual/admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessFollowers),
				sq.Or{follows, leaders, mutual, isAdmin},
			},

			// leaders (inclusive: leader/mutual/admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessLeaders),
				sq.Or{leaders, mutual, isAdmin},
			},

			// mutuals (inclusive: mutual/admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessMutuals),
				sq.Or{mutual, isAdmin},
			},

			// admins
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessAdmins),
				isAdmin,
			},
		)
	}

	// --- NODE gate ---
	nodeGate := sq.Or{
		sq.Expr("node.access IS NULL"),
		sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessPublic),
	}
	if vID != 0 {
		nodeGate = append(nodeGate,
			// registered
			sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessRegistered),

			// myself (same rule as owner gate)
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessMyself),
				relSelfOrAdminForOwner,
			},

			// followers (inclusive)
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessFollowers),
				sq.Or{follows, leaders, mutual, isAdmin},
			},

			// leaders (inclusive)
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessLeaders),
				sq.Or{leaders, mutual, isAdmin},
			},

			// mutuals (inclusive)
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

	// Hard deny if blocked either way.
	return sq.And{notBlocked, ownerGate, nodeGate}
}

// relViewerAdminsOwner checks if the viewer is an admin of the owner.
// func relViewerAdminsOwner(viewerID int64) sq.Sqlizer {
// 	return sq.Expr(`
//         EXISTS (
//           SELECT 1 FROM edges e
//           WHERE e.type = ?
//             AND e.node_a_id = ?
//             AND e.node_b_id = owner.id
//         )`,
// 		constants.EdgeActorAdminEntityType, viewerID,
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
// 		constants.EdgeFollowEntityType, viewerID,
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
// 		constants.EdgeFollowEntityType, viewerID,
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
// 		constants.EdgeBlockEntityType, viewerID, viewerID,
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
// 		viewerID, constants.EdgeActorAdminEntityType, viewerID,
// 	)
// }
