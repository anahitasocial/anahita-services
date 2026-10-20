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

	sb = sb.Where(actorFeedCondition(req.ActorId, req.Filter, req.MediumEntityTypes, req.IncludeReposts))

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

	query = query.Where(actorFeedCondition(req.ActorId, req.Filter, req.MediumEntityTypes, req.IncludeReposts))

	query = SetFeedAccessQuery(query, viewer)

	// The posts are led by what is pinned to the profile, most recently
	// pinned first. One ordering over the whole list, so a pinned post is
	// on the first page and on no other. Replies and reposts are not
	// pinned, and those lists are by date alone.
	if req.Filter == "" || req.Filter == constants.FeedFilterPosts {
		query = query.OrderBy("node.pinned DESC", "node.pinned_at DESC")
	}

	// Sort & paginate
	query = query.OrderBy("node.created_at DESC").
		Limit(req.Limit).
		Offset(req.Offset)

	return FetchNodes(repo.db, query)
}

// actorFeedCondition picks the rows of one of a profile's three lists. Who
// may see each row is the access gate's to say, not this.
//
//   - posts: what was posted on the profile, and, when asked, what its owner
//     reposted. Not replies, which are read in their thread.
//   - replies: the replies the actor wrote, wherever the threads are. A reply
//     is owned by whoever wrote it, so these are the actor's by owner as well
//     as by author. Not one that has been removed, and not one that has been
//     hidden from its thread: it is hidden for everybody, and a profile is
//     not a way round that.
//   - reposts: what the actor reposted, and nothing else.
//   - tagged: the photos and videos the actor is in, the tag approved
//     (roadmap row 103), whoever posted them; never one still waiting.
func actorFeedCondition(actorID int64, filter string, mediumTypes []string, includeReposts bool) sq.Sqlizer {
	owned := sq.Eq{"node.owner_id": actorID}

	switch filter {
	case constants.FeedFilterReplies:
		return sq.And{
			owned,
			sq.Eq{"node.type": mediumTypes},
			sq.Expr("node.root_id IS NOT NULL"),
			sq.Expr("node.deleted_at IS NULL"),
			sq.Expr(`NOT EXISTS (
        SELECT 1
        FROM edges hidden
        WHERE hidden.type = ?
          AND hidden.node_a_id = node.root_id
          AND hidden.node_b_id = node.id
      )`, constants.EdgeMediumHiddenReplyType),
		}

	case constants.FeedFilterReposts:
		return sq.And{
			owned,
			sq.Eq{"node.type": constants.NodeBaseRepostType},
		}

	case constants.FeedFilterTagged:
		return sq.And{
			sq.Expr("node.deleted_at IS NULL"),
			sq.Expr(`EXISTS (
        SELECT 1
        FROM edges tag
        WHERE tag.type = ?
          AND tag.node_a_id = ?
          AND tag.node_b_id = node.id
          AND tag.meta = ?
      )`, constants.EdgeTagPersonType, actorID, constants.TagApproved),
		}
	}

	nodeTypes := append([]string{}, mediumTypes...)
	if includeReposts {
		nodeTypes = append(nodeTypes, constants.NodeBaseRepostType)
	}

	return sq.And{
		owned,
		sq.Eq{"node.type": nodeTypes},
		sq.Expr("node.root_id IS NULL"),
	}
}
