package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
)

type SubscriptionsRelInterface interface {
	// IsSubscribedToOne checks if a given user is subscribed to a target
	IsSubscribedToOne(personID int64, targetID int64) (bool, error)

	// IsSubscribedToMany checks if a given user is subscribed to multiple targets
	IsSubscribedToMany(personID int64, targetIDs []int64) (map[int64]*entities.SubscriptionStatus, error)

	// UpdateSubscriberCount updates the subscriber count for a given target
	UpdateSubscriberCount(targetID int64) error
}

func NewSubscriptionsRelRepo(db *sql.DB) SubscriptionsRelInterface {
	return &SubscriptionsRelRepo{
		db:        db,
		dbTimeout: constants.DBTimeout,
		edgeType:  constants.EdgeBaseSubscriptionEntityType,
		edgesRepo: NewEdgesRepo(db).(*edgesRepo),
	}
}

type SubscriptionsRelRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
	edgeType  string
	edgesRepo *edgesRepo
}

func (repo *SubscriptionsRelRepo) IsSubscribedToOne(personID int64, targetID int64) (bool, error) {
	edge := entities.NewAnahitaEdge()
	edge.Type = repo.edgeType
	edge.NodeAID = personID
	edge.NodeBID = targetID
	return repo.edgesRepo.Exists(edge)
}

// IsSubscribedToMany returns one status per targetID.
// It checks whether an edge (personID -> targetID) exists in edges(type, node_a_id, node_b_id).
func (repo *SubscriptionsRelRepo) IsSubscribedToMany(personID int64, targetIDs []int64) (map[int64]*entities.SubscriptionStatus, error) {
	if len(targetIDs) == 0 {
		return map[int64]*entities.SubscriptionStatus{}, nil
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

	statuses := make(map[int64]*entities.SubscriptionStatus, len(targetIDs))
	for _, tid := range targetIDs {
		_, ok := found[tid]
		statuses[tid] = &entities.SubscriptionStatus{
			SourceID:     personID,
			TargetID:     tid,
			IsSubscribed: ok,
		}
	}
	return statuses, nil
}

func (repo *SubscriptionsRelRepo) UpdateSubscriberCount(targetID int64) error {
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

	_, err := repo.db.ExecContext(ctx, sqlStr, repo.edgeType, targetID)
	return err
}
