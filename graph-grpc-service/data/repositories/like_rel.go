package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
)

type LikesRelInterface interface {
	// IsLikedOne returns true if the node is liked by the actor
	IsLikedOne(personID int64, targetID int64) (bool, error)

	// IsLikedMany returns a map of node IDs to their liked status by the actor
	IsLikedMany(personID int64, targetIDs []int64) (map[int64]*entities.LikeStatus, error)

	// UpdateLikeCount updates like count for a node
	UpdateLikeCount(targetID int64) error
}

// NewLikes creates a new Like repository
func NewLikesRelRepo(db *sql.DB) LikesRelInterface {
	return &LikesRelRepo{
		db:        db,
		dbTimeout: constants.DBTimeout,
		edgeType:  constants.EdgeBaseLikeEntityType,
		edgesRepo: NewEdgesRepo(db).(*edgesRepo),
	}
}

type LikesRelRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
	edgeType  string
	edgesRepo *edgesRepo
}

// IsLikedOne returns true if the node is liked by the actor
func (repo *LikesRelRepo) IsLikedOne(personID int64, targetID int64) (bool, error) {
	edge := entities.NewAnahitaEdge()
	edge.Type = repo.edgeType
	edge.NodeAID = personID
	edge.NodeBID = targetID
	return repo.edgesRepo.Exists(edge)
}

// IsLikedMany returns a map of node IDs to their liked status by the actor
func (repo *LikesRelRepo) IsLikedMany(personID int64, targetIDs []int64) (map[int64]*entities.LikeStatus, error) {
	if len(targetIDs) == 0 {
		return map[int64]*entities.LikeStatus{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	sb := sq.StatementBuilder.PlaceholderFormat(sq.Question)

	where := sq.And{sq.Eq{"node_a_id": personID}, sq.Eq{"node_b_id": targetIDs}}
	where = append(where, sq.Eq{"type": repo.edgeType})

	sqlStr, args, err := sb.Select("node_b_id").From("edges").Where(where).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := make(map[int64]struct{}, len(targetIDs))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		found[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	statuses := make(map[int64]*entities.LikeStatus, len(targetIDs))
	for _, tid := range targetIDs {
		_, ok := found[tid]
		statuses[tid] = &entities.LikeStatus{
			SourceID: personID,
			TargetID: tid,
			IsLiked:  ok,
		}
	}
	return statuses, nil
}

// UpdateLikeCounts updates like counts for a node
func (repo *LikesRelRepo) UpdateLikeCount(targetID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	sqlStr := `
		UPDATE nodes AS target
		JOIN (
			SELECT COUNT(*) AS count
			FROM edges
			WHERE type = ? AND node_b_id = ?
		) AS sub
		SET target.vote_up_count = sub.count
		WHERE target.id = ?;
	`
	_, err := repo.db.ExecContext(ctx, sqlStr, repo.edgeType, targetID, targetID)
	return err
}
