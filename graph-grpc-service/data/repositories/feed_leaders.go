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
		commentEntityType: constants.NodeBaseCommentEntityType,
		followEntityType:  constants.EdgeFollowEntityType,
		likeEntityType:    constants.EdgeBaseLikeEntityType,
		db:                db,
		dbTimeout:         constants.DBTimeout,
	}
}

// FeedLeadersRepo is the FeedLeaders repository
type FeedLeadersRepo struct {
	db                *sql.DB
	commentEntityType string
	repostEntityType  string
	followEntityType  string
	likeEntityType    string
	dbTimeout         time.Duration
}

// Count returns the number of nodes
func (repo *FeedLeadersRepo) Count(req *graphpb.FeedLeadersCountRequest, viewer *entities.Viewer) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	sb := sq.Select("COUNT(*)").
		From("nodes AS node").
		LeftJoin("nodes AS owner ON owner.id = node.owner_id").
		LeftJoin("nodes AS object ON object.id = node.parent_id")

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

	// Branch 2: comments authored by leaders (parent visibility can be added separately)
	if req.IncludeComments {
		branches = append(branches, sq.And{
			sq.Eq{"node.type": repo.commentEntityType},
			sq.Expr(`EXISTS (
               SELECT 1 FROM edges le
               WHERE le.type = ?
                 AND le.node_a_id = ?
                 AND le.node_b_id = node.created_by
            )`, repo.followEntityType, viewer.ID),
		})
	}

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
	query := GetBaseSelectQuery().
		LeftJoin("nodes AS object ON object.id = node.parent_id")

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

	// Branch 2: comments authored by leaders (parent visibility can be added separately)
	if req.IncludeComments {
		branches = append(branches, sq.And{
			sq.Eq{"node.type": repo.commentEntityType},
			sq.Expr(`EXISTS (
               SELECT 1 FROM edges le
               WHERE le.type = ?
                 AND le.node_a_id = ?
                 AND le.node_b_id = node.created_by
            )`, repo.followEntityType, viewer.ID),
		})
	}

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
	query = SetFeedAccessQuery(query, viewer)

	// Sort & paginate
	query = query.OrderBy("node.created_on DESC, node.id DESC").
		Limit(req.Limit).Offset(req.Offset)

	return FetchNodes(repo.db, query)
}
