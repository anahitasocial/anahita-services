package repositories

import (
	"anahita-libs/constants"
	"database/sql"
	"time"
)

type SocialGraphCountsInterface interface {
	EditLeaderCount(actorID int64) error
	EditFollowerCount(actorID int64) error
}

func NewSocialGraphCountsRepo(db *sql.DB) SocialGraphCountsInterface {
	return &socialGraphCountsRepo{
		client:    db,
		dbTimeout: constants.DBTimeout,
	}
}

type socialGraphCountsRepo struct {
	client    *sql.DB
	dbTimeout time.Duration
}

func (r *socialGraphCountsRepo) EditLeaderCount(actorID int64) error {
	query := `
		UPDATE nodes
		SET leader_count = COALESCE((
			SELECT COUNT(node_b_id)
			FROM edges
			WHERE type = ? AND node_a_id = ?
		), '')
		WHERE id = ?`

	_, err := r.client.Exec(query, constants.EdgeFollowEntityType, actorID, actorID)
	if err != nil {
		return err
	}

	return nil
}

func (r *socialGraphCountsRepo) EditFollowerCount(actorID int64) error {
	query := `
		UPDATE nodes
		SET follower_count = COALESCE((
			SELECT COUNT(node_a_id)
			FROM edges
			WHERE type = ? AND node_b_id = ?
		), '')
		WHERE id = ?`

	_, err := r.client.Exec(query, constants.EdgeFollowEntityType, actorID, actorID)
	if err != nil {
		return err
	}

	return nil
}
