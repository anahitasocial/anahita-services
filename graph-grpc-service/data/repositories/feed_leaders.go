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

	// Active returns the actors the viewer follows that have posted in the
	// last so many hours, most recent first, counting only what the viewer
	// may see.
	Active(viewer *entities.Viewer, hours uint32, limit uint32, mediumTypes []string) ([]ActiveLeader, error)
}

// ActiveLeader is one actor the viewer follows, and what they posted lately.
type ActiveLeader struct {
	ActorID  int64
	LatestAt time.Time
	Count    uint64
}

// NewFeedLeaders creates a new FeedLeaders repository
func NewFeedLeadersRepo(db *sql.DB) FeedLeadersRepoInterface {
	return &FeedLeadersRepo{
		followEntityType: constants.EdgeFollowEntityType,
		likeEntityType:   constants.EdgeBaseLikeEntityType,
		db:               db,
		dbTimeout:        constants.DBTimeout,
	}
}

// FeedLeadersRepo is the FeedLeaders repository
type FeedLeadersRepo struct {
	db               *sql.DB
	repostEntityType string
	followEntityType string
	likeEntityType   string
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

	// Branch 3: media liked by leaders
	if req.IncludeLiked {
		branches = append(branches, sq.And{
			sq.Eq{"node.type": nodeTypes}, // slice → IN (...)
			sq.Expr(`EXISTS (
               SELECT 1
               FROM edges lk
               WHERE lk.type = ?
                 AND lk.node_b_id = node.id
                 AND EXISTS (
                    SELECT 1 FROM edges le
                    WHERE le.type = ?
                      AND le.node_a_id = ?
                      AND le.node_b_id = lk.node_a_id
                 )
            )`, repo.likeEntityType, repo.followEntityType, viewer.ID),
		})
	}

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

	// Branch 3: media liked by leaders
	if req.IncludeLiked {
		branches = append(branches, sq.And{
			sq.Eq{"node.type": req.MediumEntityTypes}, // ← uses slice
			sq.Expr(`EXISTS (
               SELECT 1
               FROM edges lk
               WHERE lk.type = ?
                 AND lk.node_b_id = node.id
                 AND EXISTS (
                    SELECT 1 FROM edges le
                    WHERE le.type = ?
                      AND le.node_a_id = ?
                      AND le.node_b_id = lk.node_a_id
                 )
            )`, repo.likeEntityType, repo.followEntityType, viewer.ID),
		})
	}

	query = query.Where(sq.Or(branches))
	// Replies are read in their thread, not in a feed.
	query = query.Where("node.root_id IS NULL")

	query = SetFeedAccessQuery(query, viewer)

	// Sort & paginate
	query = query.OrderBy("node.created_at DESC, node.id DESC").
		Limit(req.Limit).Offset(req.Offset)

	return FetchNodes(repo.db, query)
}

// Active returns the actors the viewer follows that have posted lately.
//
// Posts, and nothing else: not replies, which are read in their thread, and
// not reposts, which are somebody else's post. Each is counted only if this
// viewer may see it, through the gate the home feed uses, so an actor whose
// only recent post is one the viewer cannot see is not listed, and nobody
// learns from this that such a post exists.
//
// Not the viewer's own posts: this is about the people they follow.
func (repo *FeedLeadersRepo) Active(viewer *entities.Viewer, hours uint32, limit uint32, mediumTypes []string) ([]ActiveLeader, error) {
	if viewer == nil || viewer.ID <= 0 || len(mediumTypes) == 0 {
		return []ActiveLeader{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	sb := sq.Select("node.owner_id", "MAX(node.created_at) AS latest_at", "COUNT(*) AS total").
		From("nodes AS node").
		LeftJoin("nodes AS owner ON owner.id = node.owner_id").
		Where(sq.Eq{"node.type": mediumTypes}).
		Where("node.root_id IS NULL").
		Where("node.deleted_at IS NULL").
		Where(sq.Eq{"node.enabled": true}).
		Where(sq.NotEq{"node.owner_id": viewer.ID}).
		Where(sq.Expr("node.created_at >= UTC_TIMESTAMP() - INTERVAL ? HOUR", hours)).
		Where(sq.Expr(`EXISTS (
           SELECT 1 FROM edges le
           WHERE le.type = ?
             AND le.node_a_id = ?
             AND le.node_b_id = node.owner_id
        )`, repo.followEntityType, viewer.ID))

	sb = SetFeedAccessQuery(sb, viewer)

	sb = sb.GroupBy("node.owner_id").
		OrderBy("latest_at DESC", "node.owner_id DESC").
		Limit(uint64(limit))

	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	leaders := []ActiveLeader{}
	for rows.Next() {
		var leader ActiveLeader
		if err := rows.Scan(&leader.ActorID, &leader.LatestAt, &leader.Count); err != nil {
			return nil, err
		}
		leaders = append(leaders, leader)
	}

	return leaders, rows.Err()
}
