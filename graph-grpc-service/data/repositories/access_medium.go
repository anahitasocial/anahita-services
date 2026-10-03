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
func commentAccessCondition(viewer *entities.Viewer) sq.Sqlizer {
	return sq.And{
		relNotBlockedEitherWay(viewer.ID, contentActorRefs("node")...),
		parentPassesMediumGate(viewer),
	}
}

// parentPassesMediumGate is true when the row's parent is a post this viewer
// may see. Comments and reposts both hang off a post and are both judged by it.
//
// The post is gated in a subquery that aliases it `node` and its owner
// `owner`, the names every medium gate helper is written against, so the gate
// is the medium one exactly rather than a copy of it rewritten for other
// aliases — which is how the gates in this package have drifted before. The
// inner aliases shadow the outer ones inside the subquery only.
func parentPassesMediumGate(viewer *entities.Viewer) sq.Sqlizer {
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

	return sq.Expr("node.parent_id IN ("+postGate+")", args...)
}

// mediumAccessCondition is the gate for a post: nobody blocked either way,
// the owner's access admits the viewer, and so does the post's own.
//
// Two people pass without the post's own access being read, as they do in
// permissions.MediumPermissions.CanRead, which judges one post at a time:
//
//   - the person whose profile the post is on sees everything on it;
//   - the author sees their own post wherever they can still see the profile.
//
// Without these, nobody could list their own followers-only or mutuals-only
// posts: a person does not follow themselves, so those levels never matched.
func mediumAccessCondition(viewer *entities.Viewer) sq.Sqlizer {
	// Hard deny if blocked either way.
	notBlocked := relNotBlockedEitherWay(viewer.ID, contentActorRefs("node")...)
	ownerGate := accessGate("owner.access", viewer)
	nodeGate := accessGate("node.access", viewer)

	if viewer.ID == 0 {
		return sq.And{notBlocked, ownerGate, nodeGate}
	}

	return sq.And{
		notBlocked,
		sq.Or{
			sq.Eq{"node.owner_id": viewer.ID},
			sq.And{
				ownerGate,
				sq.Or{sq.Eq{"node.created_by": viewer.ID}, nodeGate},
			},
		},
	}
}

// accessGate admits the viewer to one access column: `owner.access` for the
// profile a post sits on, `node.access` for the post itself. Every level is
// judged by the viewer's relation to the row aliased `owner`, so the query it
// goes into must alias the post's owner that way.
//
// One function for both columns, because the two gates are the same rule and
// were written out twice — which is how the feed's copy came to check a
// different column from this one.
func accessGate(column string, viewer *entities.Viewer) sq.Sqlizer {
	vID := viewer.ID
	follows := relViewerFollowsOwner(vID)
	leaders := relOwnerFollowsViewer(vID)
	mutual := sq.And{follows, leaders}
	isAdmin := relViewerAdminsOwner(vID)

	in := func(level string) sq.Sqlizer {
		return sq.Expr("FIND_IN_SET(?, "+column+")", level)
	}

	gate := sq.Or{
		sq.Expr(column + " IS NULL"),
		in(constants.AnahitaAccessPublic),
	}
	if vID == 0 {
		return gate
	}

	return append(gate,
		in(constants.AnahitaAccessRegistered),

		// myself (person ⇒ self, non-person ⇒ admin)
		sq.And{in(constants.AnahitaAccessMyself), relSelfOrAdminForOwner(vID)},

		// followers (inclusive: follow/leader/mutual/admin)
		sq.And{in(constants.AnahitaAccessFollowers), sq.Or{follows, leaders, mutual, isAdmin}},

		// leaders (inclusive: leader/mutual/admin)
		sq.And{in(constants.AnahitaAccessLeaders), sq.Or{leaders, mutual, isAdmin}},

		// mutuals (inclusive: mutual/admin)
		sq.And{in(constants.AnahitaAccessMutuals), sq.Or{mutual, isAdmin}},

		// admins
		sq.And{in(constants.AnahitaAccessAdmins), isAdmin},
	)
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
