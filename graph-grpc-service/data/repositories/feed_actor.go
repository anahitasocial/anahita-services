package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"time"

	"context"
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

type FeedActorRepoInterface interface {
	// Count returns the number of feed items for an actor profile
	Count(req *graphpb.FeedActorCountRequest, viewer *entities.Viewer) (uint64, error)

	// Browse returns feed nodes for an actor profile
	Browse(req *graphpb.FeedActorBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)
}

func NewFeedActorRepo(db *sql.DB) FeedActorRepoInterface {
	return &FeedActorRepo{
		db:        db,
		dbTimeout: constants.DBTimeout,
	}
}

type FeedActorRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
}

// Count returns the number of feed items for an actor profile
func (repo *FeedActorRepo) Count(req *graphpb.FeedActorCountRequest, viewer *entities.Viewer) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	sb := sq.Select("COUNT(*)").
		From("nodes AS node").
		LeftJoin("nodes AS owner ON owner.id = node.owner_id")

	nodeTypes := req.MediumEntityTypes
	if req.IncludeReposts {
		nodeTypes = append(nodeTypes, constants.NodeBaseRepostType)
	}

	// Branch 0: actor’s media
	branchMedia := sq.And{
		sq.Eq{"node.type": nodeTypes}, // slice → IN (...)
		sq.Eq{"node.owner_id": req.ActorId},
	}

	// Combine branches with OR
	sb = sb.Where(sq.Or{branchMedia})

	// Apply your access gate
	sb = SetFeedAccessQuery(sb, viewer)

	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return 0, err
	}
	// log.Println("ActorFeed Count SQL:", sqlStr, args)

	var count uint64
	if err := repo.db.QueryRowContext(ctx, sqlStr, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Browse returns feed nodes for an actor profile
func (repo *FeedActorRepo) Browse(req *graphpb.FeedActorBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery()

	nodeTypes := req.MediumEntityTypes
	if req.IncludeReposts {
		nodeTypes = append(nodeTypes, constants.NodeBaseRepostType)
	}

	// Branch 1: actor’s media
	branchMedia := sq.And{
		sq.Eq{"node.type": nodeTypes},
		sq.Eq{"node.owner_id": req.ActorId},
	}

	// Combine branches with OR
	query = query.Where(sq.Or{branchMedia})

	// Apply access checks
	query = SetFeedAccessQuery(query, viewer)

	// Sort & paginate
	query = query.OrderBy("node.created_on DESC").
		Limit(req.Limit).
		Offset(req.Offset)

	return FetchNodes(repo.db, query)
}
