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

func (r *socialgraphRelRepo) getRelations(actorAIDs []int64, actorBID int64) (map[int64]*entities.ActorRelation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	rels := make(map[int64]*entities.ActorRelation, len(actorAIDs))
	// Pre-seed to avoid nil checks later (optional)
	for _, id := range actorAIDs {
		rels[id] = &entities.ActorRelation{
			ActorAId: id,
			ActorBId: actorBID,
		}
	}

	if len(actorAIDs) == 0 || actorBID <= 0 {
		// Guests or empty set => return the pre-seeded empty relations
		return rels, nil
	}

	// 1) viewer -> actorA (IsLeading)
	{
		q := sq.
			Select("e.node_b_id AS actor_id").
			From("edges e").
			Where(sq.Eq{
				"e.type":      constants.EdgeFollowEntityType,
				"e.node_a_id": actorBID,
				"e.node_b_id": actorAIDs,
			})
		sqlStr, args, err := q.ToSql()
		if err != nil {
			return nil, err
		}

		rows, err := r.db.QueryContext(ctx, sqlStr, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var actorID int64
			if err := rows.Scan(&actorID); err != nil {
				return nil, err
			}
			rels[actorID].IsLeading = true
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// 2) actorA -> actorB (IsFollowing)
	{
		q := sq.
			Select("e.node_a_id AS actor_id").
			From("edges e").
			Where(sq.Eq{
				"e.type":      constants.EdgeFollowEntityType,
				"e.node_b_id": actorBID,
				"e.node_a_id": actorAIDs,
			})
		sqlStr, args, err := q.ToSql()
		if err != nil {
			return nil, err
		}

		rows, err := r.db.QueryContext(ctx, sqlStr, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var actorID int64
			if err := rows.Scan(&actorID); err != nil {
				return nil, err
			} // <-- you were missing this
			rels[actorID].IsFollowing = true
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// 3) actorA blocks actorB (IsBlocking)
	{
		q := sq.
			Select("e.node_a_id AS actor_id").
			From("edges e").
			Where(sq.Eq{
				"e.type":      constants.EdgeBlockEntityType,
				"e.node_b_id": actorBID,
				"e.node_a_id": actorAIDs,
			})
		sqlStr, args, err := q.ToSql()
		if err != nil {
			return nil, err
		}

		rows, err := r.db.QueryContext(ctx, sqlStr, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var actorID int64
			if err := rows.Scan(&actorID); err != nil {
				return nil, err
			}
			rels[actorID].IsBlocking = true
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// 4) actorB administers actorA (HasAdmin)
	//
	// READ THE DIRECTION CAREFULLY — it is the opposite of the three relations
	// above, and it was wrong until 2026-09-07.
	//
	// An admin edge runs node_a (the administrator) -> node_b (the thing
	// administered). Every consumer of HasAdmin asks the same question: may
	// this VIEWER act on this ACTOR because they administer it. See
	// permissions/actor.go, where an actor whose access is "admins" is visible
	// when HasAdmin; permissions/medium.go and comment.go, which grant
	// follower-level reach to administrators; and feed-service's repost rules.
	//
	// The query used to select node_a_id IN actorAIDs AND node_b_id = actorBID
	// — "the ACTOR administers the VIEWER" — which is the reverse, and is a
	// relation that essentially never exists: a group does not administer a
	// person. So HasAdmin was false for everybody, and every permission
	// resting on it silently failed. A group administrator could not edit or
	// delete their own group, and admins-only content was invisible to the
	// very people it was restricted to.
	//
	// The other three relations here are genuinely actorA -> actorB and are
	// unchanged; this one is not symmetric with them, which is exactly how it
	// came to be written the wrong way round.
	{
		q := sq.
			Select("e.node_b_id AS actor_id").
			From("edges e").
			Where(sq.Eq{
				"e.type":      constants.EdgeActorAdminEntityType,
				"e.node_a_id": actorBID,
				"e.node_b_id": actorAIDs,
			})
		sqlStr, args, err := q.ToSql()
		if err != nil {
			return nil, err
		}

		rows, err := r.db.QueryContext(ctx, sqlStr, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var actorID int64
			if err := rows.Scan(&actorID); err != nil {
				return nil, err
			}
			rels[actorID].HasAdmin = true
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// 5) derive Mutual in place (no copies)
	for _, rel := range rels {
		rel.IsMutual = rel.IsLeading && rel.IsFollowing
	}

	// 5) actorB has asked to follow actorA (HasRequested)
	{
		q := sq.
			Select("e.node_b_id AS actor_id").
			From("edges e").
			Where(sq.Eq{
				"e.type":      constants.EdgeFollowRequestEntityType,
				"e.node_a_id": actorBID,
				"e.node_b_id": actorAIDs,
			})
		sqlStr, args, err := q.ToSql()
		if err != nil {
			return nil, err
		}

		rows, err := r.db.QueryContext(ctx, sqlStr, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var actorID int64
			if err := rows.Scan(&actorID); err != nil {
				return nil, err
			}
			rels[actorID].HasRequested = true
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return rels, nil
}
