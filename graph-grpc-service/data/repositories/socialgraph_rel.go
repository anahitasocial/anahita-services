package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
)

type SocialgraphRelInterface interface {
	// For many actorA IDs vs one actorB (viewer)
	GetMany(actorAIDs []int64, actorBID int64) (map[int64]*entities.ActorRelation, error)
	GetOne(actorAID int64, actorBID int64) (*entities.ActorRelation, error)
	// Which of these profiles the viewer mutes, while the mute lasts.
	MutedAmong(viewerID int64, actorIDs []int64) (map[int64]bool, error)
	// Deletes the mutes that have ended, for the sweeper.
	DeleteEndedMutes() (int64, error)
}

type socialgraphRelRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
}

func NewSocialgraphRelRepo(db *sql.DB) SocialgraphRelInterface {
	return &socialgraphRelRepo{
		db:        db,
		dbTimeout: constants.DBTimeout,
	}
}

// GetMany retrieves the relationships for many actorA IDs vs one actorB ID
func (r *socialgraphRelRepo) GetMany(actorAIDs []int64, actorBID int64) (map[int64]*entities.ActorRelation, error) {
	return r.getRelations(actorAIDs, actorBID)
}

// GetOne retrieves the relationships for one actorA vs one actorB
func (r *socialgraphRelRepo) GetOne(actorAID int64, actorBID int64) (*entities.ActorRelation, error) {
	rels, err := r.getRelations([]int64{actorAID}, actorBID)
	if err != nil {
		return nil, err
	}

	return rels[actorAID], nil
}

// getRelations reads how the viewer (actorB) and each profile (actorA)
// stand to each other: one query for each relation, run one after another,
// each closed before the next, so that a call holds one connection at a
// time (roadmap row 170: the nine were one function, each result held open
// until the end).
func (r *socialgraphRelRepo) getRelations(actorAIDs []int64, actorBID int64) (map[int64]*entities.ActorRelation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	// Every profile asked about has a relation, so that nobody checks for
	// none.
	rels := make(map[int64]*entities.ActorRelation, len(actorAIDs))
	for _, id := range actorAIDs {
		rels[id] = &entities.ActorRelation{
			ActorAId: id,
			ActorBId: actorBID,
		}
	}

	// Somebody not signed in, or nobody asked about: related to nobody.
	if len(actorAIDs) == 0 || actorBID <= 0 {
		return rels, nil
	}

	for _, relation := range relationQueries(actorAIDs, actorBID) {
		ids, err := r.idsOf(ctx, relation.query)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if rel, ok := rels[id]; ok {
				relation.set(rel)
			}
		}
	}
	if err := r.readMutes(ctx, rels, actorAIDs, actorBID); err != nil {
		return nil, err
	}

	for _, rel := range rels {
		rel.IsMutual = rel.IsLeading && rel.IsFollowing
	}
	return rels, nil
}

// relationQuery is one relation getRelations reads: which of the profiles
// it holds for, and what it sets on each.
type relationQuery struct {
	query sq.Sqlizer
	set   func(*entities.ActorRelation)
}

// relationQueries is every relation but the mutes, which say more of
// themselves (readMutes).
func relationQueries(actorAIDs []int64, actorBID int64) []relationQuery {
	// An edge from the viewer to each profile, and from each to the viewer.
	fromViewer := func(edgeType string) sq.SelectBuilder {
		return sq.Select("e.node_b_id").From("edges e").Where(sq.Eq{
			"e.type":      edgeType,
			"e.node_a_id": actorBID,
			"e.node_b_id": actorAIDs,
		})
	}
	toViewer := func(edgeType string) sq.SelectBuilder {
		return sq.Select("e.node_a_id").From("edges e").Where(sq.Eq{
			"e.type":      edgeType,
			"e.node_b_id": actorBID,
			"e.node_a_id": actorAIDs,
		})
	}

	return []relationQuery{
		// The viewer follows the profile (IsLeading).
		{fromViewer(constants.EdgeFollowEntityType), func(r *entities.ActorRelation) { r.IsLeading = true }},
		// The profile follows the viewer (IsFollowing).
		{toViewer(constants.EdgeFollowEntityType), func(r *entities.ActorRelation) { r.IsFollowing = true }},
		// The profile blocks the viewer (IsBlocking).
		{toViewer(constants.EdgeBlockEntityType), func(r *entities.ActorRelation) { r.IsBlocking = true }},
		// One of the two blocked the server the other is on, or one above
		// it (IsServerBlocked, roadmap row 152). Only a person blocks
		// servers, and only somebody elsewhere is on one, so either way it
		// is the same pair: nothing shown, no follow.
		{serverBlockedQuery(actorAIDs, actorBID), func(r *entities.ActorRelation) { r.IsServerBlocked = true }},
		// The viewer administers the profile (HasAdmin).
		//
		// READ THE DIRECTION CAREFULLY: it is the opposite of following
		// and blocking above, and it was wrong until 2026-09-07.
		//
		// An admin edge runs node_a (the administrator) -> node_b (the
		// thing administered). Every consumer of HasAdmin asks the same
		// question: may this VIEWER act on this ACTOR because they
		// administer it. See permissions/actor.go, where an actor whose
		// access is "admins" is visible when HasAdmin; permissions/medium.go
		// and comment.go, which grant follower-level reach to
		// administrators; and feed-service's repost rules.
		//
		// The query used to select node_a_id IN actorAIDs AND node_b_id =
		// actorBID, "the ACTOR administers the VIEWER", which is the
		// reverse, and is a relation that essentially never exists: a group
		// does not administer a person. So HasAdmin was false for
		// everybody, and every permission resting on it silently failed. A
		// group administrator could not edit or delete their own group, and
		// admins-only content was invisible to the very people it was
		// restricted to.
		{fromViewer(constants.EdgeActorAdminEntityType), func(r *entities.ActorRelation) { r.HasAdmin = true }},
		// The viewer has asked to follow the profile (HasRequested).
		{fromViewer(constants.EdgeFollowRequestEntityType), func(r *entities.ActorRelation) { r.HasRequested = true }},
		// The viewer is invited to follow the profile, and has not
		// answered (HasInvite).
		{
			fromViewer(constants.EdgeActorInviteEntityType).
				Where(sq.Eq{"e.meta": constants.InviteMetaPending}).
				Where("e.ends_at > NOW()"),
			func(r *entities.ActorRelation) { r.HasInvite = true },
		},
		// The profile is an event open to its host's followers, and the
		// viewer follows the group hosting it (ViaHost).
		{
			sq.Select("host.node_b_id").
				From("edges host").
				Join("events ev ON ev.node_id = host.node_b_id AND ev.open_to_host_followers = 1").
				Join("edges follows ON follows.node_b_id = host.node_a_id").
				Where(sq.Eq{
					"host.type":         constants.EdgeActorHostEntityType,
					"host.node_b_id":    actorAIDs,
					"follows.type":      constants.EdgeFollowEntityType,
					"follows.node_a_id": actorBID,
				}),
			func(r *entities.ActorRelation) { r.ViaHost = true },
		},
	}
}

// serverBlockedQuery is the profiles among actorAIDs that blocked the
// viewer's server, or whose server the viewer blocked.
func serverBlockedQuery(actorAIDs []int64, actorBID int64) sq.Sqlizer {
	in := sq.Placeholders(len(actorAIDs))
	query := `
SELECT b.person_id
FROM person_domain_blocks b
JOIN nodes nb ON nb.id = ?
WHERE b.person_id IN (` + in + `)
  AND ` + onBlockedServer("nb.origin_domain") + `
UNION
SELECT na.id
FROM nodes na
JOIN person_domain_blocks b ON b.person_id = ?
WHERE na.id IN (` + in + `)
  AND ` + onBlockedServer("na.origin_domain")
	args := []any{actorBID}
	for _, id := range actorAIDs {
		args = append(args, id)
	}
	args = append(args, actorBID)
	for _, id := range actorAIDs {
		args = append(args, id)
	}
	return sq.Expr(query, args...)
}

// idsOf runs a query of profile ids and closes it before answering.
func (r *socialgraphRelRepo) idsOf(ctx context.Context, query sq.Sqlizer) ([]int64, error) {
	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// readMutes marks the profiles that mute the viewer, while the mute lasts
// (IsMuting): whether it hides the viewer's notifications too, and when it
// ends.
func (r *socialgraphRelRepo) readMutes(ctx context.Context, rels map[int64]*entities.ActorRelation, actorAIDs []int64, actorBID int64) error {
	sqlStr, args, err := sq.
		Select("e.node_a_id", "e.meta", "e.ends_at").
		From("edges e").
		Where(sq.Eq{
			"e.type":      constants.EdgeMuteEntityType,
			"e.node_b_id": actorBID,
			"e.node_a_id": actorAIDs,
		}).
		Where(muteLasts("e")).
		ToSql()
	if err != nil {
		return err
	}
	rows, err := r.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			actorID int64
			meta    string
			endsAt  sql.NullTime
		)
		if err := rows.Scan(&actorID, &meta, &endsAt); err != nil {
			return err
		}
		rel, ok := rels[actorID]
		if !ok {
			continue
		}
		rel.IsMuting = true
		rel.MuteHidesNotifications = meta != constants.MuteMetaNotifications
		if endsAt.Valid {
			rel.MuteEndsAt = endsAt.Time
		}
	}
	return rows.Err()
}
