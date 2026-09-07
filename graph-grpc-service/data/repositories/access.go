package repositories

import (
	"strings"

	"anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// SetAccessQuery returns the access query for media repository
func SetAccessQuery(sb sq.SelectBuilder, viewer *entities.Viewer) sq.SelectBuilder {
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

	return sb.Where(ownerGate).Where(nodeGate)
}

// relViewerAdminsOwner checks if the viewer is an admin of the owner.
func relViewerAdminsOwner(viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        EXISTS (
          SELECT 1 FROM edges e
          WHERE e.type = ?
            AND e.node_a_id = ?
            AND e.node_b_id = owner.id
        )`,
		constants.EdgeActorAdminEntityType, viewerID,
	)
}

// relViewerFollowsOwner checks if the viewer follows the owner.
func relViewerFollowsOwner(viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        EXISTS (
          SELECT 1 FROM edges e
          WHERE e.type = ?
            AND e.node_a_id = ?
            AND e.node_b_id = owner.id
        )`,
		constants.EdgeFollowEntityType, viewerID,
	)
}

// relOwnerFollowsViewer checks if the owner follows the viewer.
func relOwnerFollowsViewer(viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        EXISTS (
          SELECT 1 FROM edges e
          WHERE e.type = ?
            AND e.node_a_id = owner.id
            AND e.node_b_id = ?
        )`,
		constants.EdgeFollowEntityType, viewerID,
	)
}

// relNotBlockedEitherWay hides a row when the viewer has blocked, or been
// blocked by, ANY of the actors the row belongs to.
//
// actorRefs are SQL expressions on the row that yield an actor id. Callers
// pass every actor the row is attributable to, because blocking somebody means
// not seeing their posts anywhere — not merely not seeing their profile:
//
//	content:  node.owner_id (the actor it was posted to)
//	          node.created_by (who wrote it — the one that matters when
//	          somebody posts into a group they do not own)
//	          node.id (harmless, and correct when the row IS an actor)
//	actors:   node.id alone. An actor's created_by is whoever created it, and
//	          blocking a group's creator should not hide the group.
//
// It used to take no arguments and compare against node.id only. In an actor
// listing node IS the actor so that worked, but in media, feed, search and
// polymorphic queries node is the content item — a photo, an article — and no
// block edge ever references a photo's id. The predicate could not match, so
// blocking hid the person and left every one of their posts visible.
//
// Blocks are symmetric here: A blocking B hides B from A AND A from B.
//
// COALESCE rather than relying on NULL semantics inside IN: a NULL owner_id
// makes the whole IN comparison NULL rather than false, which happens to give
// the right answer but only by way of three-valued logic nobody should have to
// re-derive when reading this.
//
// On cost: this is correlated, but each branch pins the viewer with an
// equality — indexed by uc_edge on (type, node_a_id, ...) one way and by the
// node_b_id index the other — so it probes the viewer's own block list, which
// is small, rather than scanning edges per row.
func relNotBlockedEitherWay(viewerID int64, actorRefs ...string) sq.Sqlizer {
	if len(actorRefs) == 0 {
		actorRefs = []string{"node.id"}
	}

	refs := make([]string, 0, len(actorRefs))
	for _, ref := range actorRefs {
		refs = append(refs, "COALESCE("+ref+", 0)")
	}
	list := strings.Join(refs, ", ")

	return sq.Expr(`
        NOT EXISTS (
          SELECT 1 FROM edges e
          WHERE e.type = ?
            AND (
               (e.node_a_id = ? AND e.node_b_id IN (`+list+`))
            OR (e.node_b_id = ? AND e.node_a_id IN (`+list+`))
            )
        )`,
		constants.EdgeBlockEntityType, viewerID, viewerID,
	)
}

// contentActorRefs are the actors a content row is attributable to.
func contentActorRefs(alias string) []string {
	return []string{alias + ".id", alias + ".owner_id", alias + ".created_by"}
}

// relSelfOrAdminForOwner checks if the viewer is the owner itself or an admin of the owner.
// If owner is a person: myself = (viewer is the owner)
// Else (group/event/...): myself = (viewer admins the owner)
func relSelfOrAdminForOwner(viewerID int64) sq.Sqlizer {
	return sq.Expr(`
        (
          (owner.type LIKE 'node.actor.person-service.person.%' AND owner.id = ?)
          OR
          (owner.type NOT LIKE 'node.actor.person-service.person.%' AND EXISTS (
              SELECT 1 FROM edges e
              WHERE e.type = ?
                AND e.node_a_id = ?
                AND e.node_b_id = owner.id
          ))
        )`,
		viewerID, constants.EdgeActorAdminEntityType, viewerID,
	)
}
