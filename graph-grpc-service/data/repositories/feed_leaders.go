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

type FeedLeadersRepoInterface interface {
	// Count returns the number of Feeds
	Count(req *graphpb.FeedLeadersCountRequest, viewer *entities.Viewer) (uint64, error)

	// Browse returns a list of Feeds
	Browse(req *graphpb.FeedLeadersBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)
}

// NewFeedLeaders creates a new FeedLeaders repository
func NewFeedLeadersRepo(db *sql.DB) FeedLeadersRepoInterface {
	return &FeedLeadersRepo{
		followEntityType: constants.EdgeFollowEntityType,
		db:               db,
		dbTimeout:        constants.DBTimeout,
	}
}

// FeedLeadersRepo is the FeedLeaders repository: the Following feed, the
// viewer's own posts and the posts and reposts of the profiles they follow,
// reached through follow edges and nothing else. What those profiles only
// liked is not in it (2026-10-29); the request's include_liked is not read.
type FeedLeadersRepo struct {
	db               *sql.DB
	repostEntityType string
	followEntityType string
	dbTimeout        time.Duration
}

// Count returns the number of nodes
func (repo *FeedLeadersRepo) Count(req *graphpb.FeedLeadersCountRequest, viewer *entities.Viewer) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	sb := sq.Select("COUNT(*)").
		From("nodes AS node").
		LeftJoin("nodes AS owner ON owner.id = node.owner_id")

	nodeTypes := req.MediumEntityTypes
	if req.IncludeReposts {
		nodeTypes = append(nodeTypes, constants.NodeBaseRepostType)
	}

	// Branch 0: viewer's media
	branchViewerMedia := sq.And{
		sq.Eq{"node.type": nodeTypes}, // slice → IN (...)
		sq.Eq{"node.owner_id": viewer.ID},
	}

	branches := []sq.Sqlizer{branchViewerMedia}

	// Branch 1: leaders' media
	branchMedia := sq.And{
		sq.Eq{"node.type": nodeTypes}, // slice → IN (...)
		sq.Expr(`EXISTS (
           SELECT 1 FROM edges le
           WHERE le.type = ?
             AND le.node_a_id = ?
             AND le.node_b_id = node.owner_id
        )`, repo.followEntityType, viewer.ID),
	}

	branches = append(branches, branchMedia)

	sb = sb.Where(sq.Or(branches))
	// Replies are read in their thread, not in a feed.
	sb = sb.Where("node.root_id IS NULL")

	sb = SetFeedAccessQuery(sb, viewer)

	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return 0, err
	}

	// log.Println("Leaders Count SQL:", sqlStr, args)

	var count uint64
	if err := repo.db.QueryRowContext(ctx, sqlStr, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Browse returns a list of Feeds
func (repo *FeedLeadersRepo) Browse(req *graphpb.FeedLeadersBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery()

	nodeTypes := req.MediumEntityTypes
	if req.IncludeReposts {
		nodeTypes = append(nodeTypes, constants.NodeBaseRepostType)
	}

	// Branch 0: viewer's media
	branchViewerMedia := sq.And{
		sq.Eq{"node.type": nodeTypes}, // slice → IN (...)
		sq.Eq{"node.owner_id": viewer.ID},
	}

	branches := []sq.Sqlizer{branchViewerMedia}

	// Branch 1: leaders' media
	branchMedia := sq.And{
		sq.Eq{"node.type": nodeTypes}, // slice → IN (...)
		sq.Expr(`EXISTS (
           SELECT 1 FROM edges le
           WHERE le.type = ?
             AND le.node_a_id = ?
             AND le.node_b_id = node.owner_id
        )`, repo.followEntityType, viewer.ID),
	}

	branches = append(branches, branchMedia)

	query = query.Where(sq.Or(branches))
	// Replies are read in their thread, not in a feed.
	query = query.Where("node.root_id IS NULL")

	query = SetFeedAccessQuery(query, viewer)

	// Sort & paginate
	query = query.OrderBy("node.created_at DESC, node.id DESC").
		Limit(req.Limit).Offset(req.Offset)

	return FetchNodes(repo.db, query)
}
