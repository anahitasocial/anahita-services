package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// SetSearchAccessQuery returns the access query for media repository
func SetSearchAccessQuery(sb sq.SelectBuilder, viewer *entities.Viewer, includeCommentParentNode bool) sq.SelectBuilder {
	if viewer.IsAdmin() {
		return sb
	}

	vID := viewer.ID

	// Common relation shortcuts for "owner" and "node".
	//
	// ONE block predicate, not two. These were previously separate names for
	// byte-identical expressions: the helper had lost its alias parameter, so
	// "notBlocked" and "notBlocked" both compiled to the same
	// node.id comparison and the query carried it twice.
	//
	// The single predicate below covers the owner and the author as well as
	// the row itself, which is what the two names were reaching for.
	notBlocked := relNotBlockedEitherWay(vID, contentActorRefs("node")...)

	ownerFollows := relViewerFollows("owner", vID)
	ownerLeads := relAliasFollowsViewer("owner", vID)
	ownerMutual := sq.And{ownerFollows, ownerLeads}
	ownerAdmin := relViewerAdmins("owner", vID)
	ownerSelfAdm := relSelfOrAdmin("owner", vID)

	nodeFollows := relViewerFollows("node", vID)
	nodeLeads := relAliasFollowsViewer("node", vID)
	nodeMutual := sq.And{nodeFollows, nodeLeads}
	nodeAdmin := relViewerAdmins("node", vID)
	nodeSelfAdm := relSelfOrAdmin("node", vID)

	// Hard deny on blocks. ONCE — contentActorRefs already spans the row, its
	// owner and its author, which is what "applied to both owner and node"
	// was reaching for. Applying it twice put a byte-identical predicate in
	// every search query, left over from when there really were two of them
	// under two names.
	sb = sb.Where(notBlocked)

	// ---- OWNER gate (coarse gate) ----
	ownerGate := sq.Or{
		sq.Expr("owner.access IS NULL"),
		sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessPublic),
	}
	if vID != 0 {
		ownerGate = append(ownerGate,
			// registered
			sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessRegistered),

			// myself (person ⇒ self; non-person ⇒ admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessMyself),
				ownerSelfAdm,
			},

			// followers (inclusive: follow/leader/mutual/admin)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessFollowers),
				sq.Or{ownerFollows, ownerLeads, ownerMutual, ownerAdmin},
			},

			// leaders (inclusive)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessLeaders),
				sq.Or{ownerLeads, ownerMutual, ownerAdmin},
			},

			// mutuals (inclusive)
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessMutuals),
				sq.Or{ownerMutual, ownerAdmin},
			},

			// admins
			sq.And{
				sq.Expr("FIND_IN_SET(?, owner.access)", constants.AnahitaAccessAdmins),
				ownerAdmin,
			},
		)
	}

	// ---- NODE gate (fine gate) ----
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
				nodeSelfAdm,
			},

			// followers (inclusive)
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessFollowers),
				sq.Or{nodeFollows, nodeLeads, nodeMutual, nodeAdmin},
			},

			// leaders (inclusive)
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessLeaders),
				sq.Or{nodeLeads, nodeMutual, nodeAdmin},
			},

			// mutuals (inclusive)
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessMutuals),
				sq.Or{nodeMutual, nodeAdmin},
			},

			// admins
			sq.And{
				sq.Expr("FIND_IN_SET(?, node.access)", constants.AnahitaAccessAdmins),
				nodeAdmin,
			},
		)
	}

	// A post is judged by its owner and its own audience. A reply has no
	// audience of its own and is owned by its author, so it is judged by the
	// post at the top of its thread and by nothing else.
	sb = sb.Where(postOrReply(sq.And{ownerGate, nodeGate}, viewer))

	// ---- Optional: comment parent gate ----
	if includeCommentParentNode {
		// The comment's parent is a different row with its own owner and
		// author, so it needs its own predicate against those aliases —
		// previously this was the node predicate again under another name.
		cpNotBlocked := relNotBlockedEitherWay(vID, contentActorRefs("comment_parent_node")...)
		cpFollows := relViewerFollows("comment_parent_node", vID)
		cpLeads := relAliasFollowsViewer("comment_parent_node", vID)
		cpMutual := sq.And{cpFollows, cpLeads}
		cpAdmin := relViewerAdmins("comment_parent_node", vID)
		cpSelfAdm := relSelfOrAdmin("comment_parent_node", vID)

		// Deny comments if the relationship with the comment's parent is blocked
		sb = sb.Where(cpNotBlocked)

		cpGate := sq.Or{
			sq.Expr("comment_parent_node.access IS NULL"),
			sq.Expr("FIND_IN_SET(?, comment_parent_node.access)", constants.AnahitaAccessPublic),
		}
		if vID != 0 {
			cpGate = append(cpGate,
				sq.Expr("FIND_IN_SET(?, comment_parent_node.access)", constants.AnahitaAccessRegistered),

				sq.And{
					sq.Expr("FIND_IN_SET(?, comment_parent_node.access)", constants.AnahitaAccessMyself),
					cpSelfAdm,
				},

				sq.And{
					sq.Expr("FIND_IN_SET(?, comment_parent_node.access)", constants.AnahitaAccessFollowers),
					sq.Or{cpFollows, cpLeads, cpMutual, cpAdmin},
				},

				sq.And{
					sq.Expr("FIND_IN_SET(?, comment_parent_node.access)", constants.AnahitaAccessLeaders),
					sq.Or{cpLeads, cpMutual, cpAdmin},
				},

				sq.And{
					sq.Expr("FIND_IN_SET(?, comment_parent_node.access)", constants.AnahitaAccessMutuals),
					sq.Or{cpMutual, cpAdmin},
				},

				sq.And{
					sq.Expr("FIND_IN_SET(?, comment_parent_node.access)", constants.AnahitaAccessAdmins),
					cpAdmin,
				},
			)
		}

		sb = sb.Where(cpGate)
	}

	return sb
}

func relViewerFollows(alias string, viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        EXISTS (
          SELECT 1 FROM edges e
          WHERE e.type = ?
            AND e.node_a_id = ?
            AND e.node_b_id = `+alias+`.id
        )`, constants.EdgeFollowEntityType, viewerID)
}

func relAliasFollowsViewer(alias string, viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        EXISTS (
          SELECT 1 FROM edges e
          WHERE e.type = ?
            AND e.node_a_id = `+alias+`.id
            AND e.node_b_id = ?
        )`, constants.EdgeFollowEntityType, viewerID)
}

func relViewerAdmins(alias string, viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        EXISTS (
          SELECT 1 FROM edges e
          WHERE e.type = ?
            AND e.node_a_id = ?
            AND e.node_b_id = `+alias+`.id
        )`, constants.EdgeActorAdminEntityType, viewerID)
}

// func relNotBlockedEitherWay(alias string, viewerID int64) sq.Sqlizer {
// 	return sq.Expr(`
//         NOT EXISTS (
//           SELECT 1 FROM edges e
//           WHERE e.type = ?
//             AND (
//                  (e.node_a_id = ? AND e.node_b_id = `+alias+`.id)
//               OR (e.node_b_id = ? AND e.node_a_id = `+alias+`.id)
//             )
//         )`, constants.EdgeBlockEntityType, viewerID, viewerID)
// }

// If alias is a person actor: myself ⇒ viewer == alias.id
// Else (group/event/...): myself ⇒ viewer admins alias
func relSelfOrAdmin(alias string, viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        (
          (`+alias+`.type LIKE 'node.actor.person-service.person.%' AND `+alias+`.id = ?)
          OR
          (`+alias+`.type NOT LIKE 'node.actor.person-service.person.%' AND EXISTS (
              SELECT 1 FROM edges e
              WHERE e.type = ?
                AND e.node_a_id = ?
                AND e.node_b_id = `+alias+`.id
          ))
        )`, viewerID, constants.EdgeActorAdminEntityType, viewerID)
}
