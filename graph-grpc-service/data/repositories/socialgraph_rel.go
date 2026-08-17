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

	// 4) actorA has actorB as admin (HasAdmin)
	{
		q := sq.
			Select("e.node_a_id AS actor_id").
			From("edges e").
			Where(sq.Eq{
				"e.type":      constants.EdgeActorAdminEntityType,
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

	return rels, nil
}
