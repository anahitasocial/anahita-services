package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"database/sql"
	"strings"
	"time"

	"graph/ranking"

	sq "github.com/Masterminds/squirrel"
)

func NewGraphRepo(db *sql.DB) GraphRepoInterface {
	return &graphRepo{
		db:        db,
		dbTimeout: constants.DBTimeout,
		ranking:   ranking.FromEnv(),
	}
}

type graphRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
	// The weights Most relevant is ordered by, as search's (package ranking).
	ranking *ranking.Source
}

// BrowseNeighbours returns a paginated list of 1-hop neighbors for a given node.
//
// This method:
//   - supports filtering by edge types and direction (IN, OUT, BOTH)
//   - supports limit + offset pagination
//   - performs permission checks based on the viewer
//
// Use this for building lists of related nodes, such as friends, followers,
// group members, etc.
func (r *graphRepo) BrowseNeighbours(req *graphpb.BrowseNeighboursRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery()

	// Add joins based on direction
	switch req.Direction {
	case constants.GraphQueryDirectionInbound:
		query = query.LeftJoin(
			"edges AS edge ON edge.node_a_id = node.id",
		).Where(sq.Eq{"edge.node_b_id": req.NodeId})
	case constants.GraphQueryDirectionOutbound:
		query = query.LeftJoin(
			"edges AS edge ON edge.node_b_id = node.id",
		).Where(sq.Eq{"edge.node_a_id": req.NodeId})
	case constants.GraphQueryDirectionBoth:
		query = query.LeftJoin(
			"edges AS edge ON (edge.node_a_id = node.id OR edge.node_b_id = node.id)",
		).Where(sq.Or{
			sq.Eq{"edge.node_a_id": req.NodeId},
			sq.Eq{"edge.node_b_id": req.NodeId},
		})
	default:
		query = query.LeftJoin(
			"edges AS edge ON edge.node_a_id = node.id",
		).Where(sq.Eq{"edge.node_b_id": req.NodeId})
	}

	// Filter by edge types if provided
	if len(req.EdgeTypes) > 0 {
		query = query.Where(sq.Eq{"edge.type": req.EdgeTypes})
	}

	query = setNeighbourSearchQuery(query, req.Search)
	query = setFollowedByQuery(query, req.FollowedBy)
	query = setEdgeMetaQuery(query, req.EdgeMeta, req.EdgeMetaNot)

	// ALWAYS, not `if req.AccessCheck` — see the note in nodes.go. The flag is
	// a protobuf bool that arrives false unless a caller sets it, and none do.
	//
	// This is what let a restricted actor into a followers list: browsing
	// socialgraph/:id/leaders returned an admins-only profile to a logged-out
	// visitor, at offset 100 where a first-page check did not see it.
	//
	// The gate was commented out rather than wrong, and the reason is visible
	// in the count query below: SetPolymorphicAccessQuery reads owner.access,
	// which the count query had no alias for. The browse query does —
	// GetBaseSelectQuery joins it — so only the count needed the join added.
	query = setNeighbourAccessQuery(query, viewer, req.EdgeTypes)
	// A hashtag's or a place's posts are a listing (listed.go).
	query = listedQuery(query)
	query = setBlockedLikersQuery(query, req.EdgeTypes, req.Direction)

	// Add ordering
	if req.Ordering == constants.OrderingConnected {
		// By when the connection was made: who followed most lately, not
		// who joined the site most lately. Newest first unless the oldest
		// are asked for. The oldest edges of all have no date, and an
		// order by date puts them at the old end either way.
		if req.OrderingDir == constants.DirAsc {
			query = query.OrderBy("edge.created_at ASC", "edge.id ASC")
		} else {
			query = query.OrderBy("edge.created_at DESC", "edge.id DESC")
		}
	} else if req.Ordering != "" {
		if req.Ordering == constants.OrderingPopular {
			query = query.OrderBy("(node.repost_count + node.comment_count + node.vote_up_count + node.subscriber_count + node.follower_count + node.citation_count) DESC")
		} else if req.Ordering == constants.OrderingRecent {
			query = query.OrderBy("node.created_at DESC")
		} else if req.Ordering == constants.OrderingRelevant {
			// What people have found useful lately, as search's Most
			// relevant ranks it, with the viewer's own leanings; the newest
			// first among equals.
			weights := r.ranking.Weights()
			if viewer != nil && viewer.ID > 0 {
				if steps, err := readPreferences(r.db, r.dbTimeout, viewer.ID); err == nil && len(steps) > 0 {
					weights = weights.Adjusted(steps)
				}
			}
			query = query.OrderByClause(sq.ConcatExpr(Standing(weights, viewer), " DESC")).
				OrderBy("node.created_at DESC", "node.id DESC")
		} else if column := safeOrderColumn(req.Ordering); column != "" {
			// Checked here as well as at the API boundary: ORDER BY is
			// concatenated, and gRPC callers never passed through that
			// boundary. See ordering.go.
			if req.OrderingDir == constants.DirAsc {
				query = query.OrderBy(column + " ASC")
			} else {
				query = query.OrderBy(column + " DESC")
			}
		}
	}

	query = query.Limit(req.Limit).Offset(req.Offset)

	return FetchNodes(r.db, query)
}

// CountNeighbours returns the total number of 1-hop neighbors that match
func (r *graphRepo) CountNeighbours(req *graphpb.BrowseNeighboursRequest, viewer *entities.Viewer) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	// Get total count first - use a subquery to count distinct node IDs
	query := sq.Select("COUNT(DISTINCT node.id)").From("nodes AS node")

	// Add the same joins and where conditions as the main query
	switch req.Direction {
	case constants.GraphQueryDirectionInbound:
		query = query.LeftJoin(
			"edges AS edge ON edge.node_a_id = node.id",
		).Where(sq.Eq{"edge.node_b_id": req.NodeId})
	case constants.GraphQueryDirectionOutbound:
		query = query.LeftJoin(
			"edges AS edge ON edge.node_b_id = node.id",
		).Where(sq.Eq{"edge.node_a_id": req.NodeId})
	case constants.GraphQueryDirectionBoth:
		query = query.LeftJoin(
			"edges AS edge ON (edge.node_a_id = node.id OR edge.node_b_id = node.id)",
		).Where(sq.Or{
			sq.Eq{"edge.node_a_id": req.NodeId},
			sq.Eq{"edge.node_b_id": req.NodeId},
		})
	default:
		query = query.LeftJoin(
			"edges AS edge ON edge.node_a_id = node.id",
		).Where(sq.Eq{"edge.node_b_id": req.NodeId})
	}

	// Filter by edge types if provided
	if len(req.EdgeTypes) > 0 {
		query = query.Where(sq.Eq{"edge.type": req.EdgeTypes[0]})
	}

	// The same narrowing as the list, or the total is of a different list.
	query = setNeighbourSearchQuery(query, req.Search)
	query = setFollowedByQuery(query, req.FollowedBy)
	query = setEdgeMetaQuery(query, req.EdgeMeta, req.EdgeMetaNot)

	// countQuery = countQuery.Where(sq.Eq{"node.enabled": req.Enabled})

	// The owner join exists only so the gate below can read owner.access. The
	// browse query gets it from GetBaseSelectQuery; this one is built by hand
	// and had no owner alias, which is why the gate could not simply be
	// uncommented here.
	//
	// A LEFT JOIN, so a node without an owner still counts — an actor has
	// none, and the gate treats a NULL owner as passing.
	query = query.LeftJoin("nodes AS owner ON owner.id = node.owner_id")
	query = setNeighbourAccessQuery(query, viewer, req.EdgeTypes)
	// A hashtag's or a place's posts are a listing (listed.go).
	query = listedQuery(query)
	query = setBlockedLikersQuery(query, req.EdgeTypes, req.Direction)

	countSql, countArgs, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	// log.Println("Count SQL:", countSql, "Args:", countArgs) // Debug log for count query

	var total uint64
	err = r.db.QueryRowContext(ctx, countSql, countArgs...).Scan(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
}

// BrowseConnectionNodes returns the 1-hop neighbors connected by a single edge type.
//
// This mirrors BrowseNeighbours but is specialized for a single edge type.
// It is useful for scenarios like fetching friends, followers, or members
// of a specific group.
func (r *graphRepo) BrowseConnectionNodes(req *graphpb.BrowseConnectionNodesRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	req2 := &graphpb.BrowseNeighboursRequest{
		NodeId:      req.NodeId,
		EdgeTypes:   []string{req.EdgeType},
		Direction:   req.Direction,
		Limit:       req.Limit,
		Offset:      req.Offset,
		AccessCheck: req.AccessCheck,
		Ordering:    req.Ordering,
		OrderingDir: req.OrderingDir,
		Search:      req.Search,
		FollowedBy:  req.FollowedBy,
		EdgeMeta:    req.EdgeMeta,
		EdgeMetaNot: req.EdgeMetaNot,
	}
	return r.BrowseNeighbours(req2, viewer)
}

// setNeighbourSearchQuery keeps the neighbours whose name or alias holds what
// was typed: a follower found by name. Nothing typed keeps them all.
func setNeighbourSearchQuery(query sq.SelectBuilder, search string) sq.SelectBuilder {
	search = sanitizeSearchString(search)
	if search == "" {
		return query
	}

	likePattern := "%" + search + "%"

	return query.Where(sq.Or{
		sq.Like{"node.name": likePattern},
		sq.Like{"node.alias": likePattern},
	})
}

// setFollowedByQuery keeps the neighbours that one actor follows.
//
// Asked of a profile's followers, it gives who that actor and the profile
// have in common: the people the actor follows who follow the profile. It is
// done in the query so that the whole of it is found, paged and counted.
// It used to be worked out by comparing one page of each side, which missed
// everybody past the first page of either.
func setFollowedByQuery(query sq.SelectBuilder, actorID int64) sq.SelectBuilder {
	if actorID <= 0 {
		return query
	}

	return query.Where(sq.Expr(`
        EXISTS (
          SELECT 1 FROM edges common
          WHERE common.type = ?
            AND common.node_a_id = ?
            AND common.node_b_id = node.id
        )`,
		constants.EdgeFollowEntityType, actorID,
	))
}

// setEdgeMetaQuery keeps the neighbours whose edge says exactly this in its
// meta, or the ones whose edge does not.
//
// An answer to an event is kept whole in the follow edge's meta, so the
// people going are the edges that say so, and the ones who said maybe are
// every other follow, an edge with nothing in it included.
func setEdgeMetaQuery(query sq.SelectBuilder, is string, isNot string) sq.SelectBuilder {
	if is != "" {
		query = query.Where(sq.Eq{"edge.meta": is})
	}
	if isNot != "" {
		query = query.Where(sq.Or{
			sq.Expr("edge.meta IS NULL"),
			sq.NotEq{"edge.meta": isNot},
		})
	}

	return query
}

// CountConnectionNodes returns the total number of 1-hop neighbors connected by a single edge type.
//
// This mirrors CountNeighbours but is specialized for a single edge type.
// It is useful for scenarios like counting friends, followers, or members
// of a specific group.
func (r *graphRepo) CountConnectionNodes(req *graphpb.BrowseConnectionNodesRequest, viewer *entities.Viewer) (uint64, error) {
	req2 := &graphpb.BrowseNeighboursRequest{
		NodeId:      req.NodeId,
		EdgeTypes:   []string{req.EdgeType},
		Direction:   req.Direction,
		AccessCheck: req.AccessCheck,
		Search:      req.Search,
		FollowedBy:  req.FollowedBy,
		EdgeMeta:    req.EdgeMeta,
		EdgeMetaNot: req.EdgeMetaNot,
	}
	return r.CountNeighbours(req2, viewer)
}

// BrowseOrderedByDegree returns nodes ordered by their degree (number of connections).
//
// This method:
//   - supports filtering by node type
//   - supports filtering by edge types and direction
//   - supports limit + offset pagination
//   - performs permission checks based on the viewer
//
// Use this for discovery features, popular nodes, or highly connected entities.
func (r *graphRepo) BrowseOrderedByDegree(req *graphpb.BrowseOrderedByDegreeRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	// ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	// defer cancel()

	query := GetBaseSelectQuery()

	// Add degree count to the base query
	query = query.Join("edges AS edge ON " + r.buildDegreeJoinCondition(req.Direction)).
		GroupBy("node.id").
		OrderBy("COUNT(edge.id) DESC")

	query = query.Limit(req.Limit).Offset(req.Offset)

	// Node type filter
	if req.NodeType != "" {
		query = query.Where(sq.Eq{"node.type": req.NodeType})
	}

	// Edge type filter (if provided)
	if len(req.EdgeTypes) > 0 {
		query = query.Where(sq.Eq{"edge.type": req.EdgeTypes})
	}

	// Apply viewer access permissions
	if req.AccessCheck {
		query = SetAccessQuery(query, viewer)
	}

	return FetchNodes(r.db, query)
}

// ReadNeighbourhoods retrieves the full 1-hop neighborhood for a set of nodes.
//
// This method:
//   - supports filtering by edge types and direction
//   - returns all connected nodes and edges in a structured format
//   - performs permission checks based on the viewer
//
// Use this for visualizing local graph structures, ego networks,
// or detailed relationship views.
func (r *graphRepo) ReadNeighbourhoods(req *graphpb.ReadNeighbourhoodRequest, viewer *entities.Viewer) (*graphpb.ReadNeighbourhoodResponse, error) {
	// Implementation goes here
	return &graphpb.ReadNeighbourhoodResponse{}, nil
}

// Traverse performs multi-hop traversals starting from a set of nodes.
//
// This method:
//   - supports filtering by edge types and direction at each hop
//   - allows specifying max depth and traversal strategies (BFS, DFS)
//   - performs permission checks based on the viewer
//
// Use this for recommendations, pathfinding, or exploring extended
// relationships in the graph.
func (r *graphRepo) Traverse(req *graphpb.TraverseRequest, viewer *entities.Viewer) (*graphpb.TraverseResponse, error) {
	// Implementation goes here
	return &graphpb.TraverseResponse{}, nil
}

// ReadShortestPath finds the shortest path between two nodes.
//
// This method:
//   - supports filtering by edge types and direction
//   - returns the sequence of nodes and edges forming the shortest path
//   - performs permission checks based on the viewer
//
// Use this for connection discovery, relationship insights,
// or path analysis within the graph.
func (r *graphRepo) ReadShortestPath(req *graphpb.ReadShortestPathRequest, viewer *entities.Viewer) (*graphpb.ReadShortestPathResponse, error) {
	// Implementation goes here
	return &graphpb.ReadShortestPathResponse{}, nil
}

// Trending returns the tags, hashtags or places, that the most people
// posted with in a window of time that ends now: the people who wrote a
// public post carrying it in that time, each once. The edge types say
// which tag; the direction is not read, a tag being node_a of its edges.
func (r *graphRepo) Trending(req *graphpb.TrendingRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	// ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	// defer cancel()

	cutoff := trendingCutoff(req.TimeWindow)

	// What is trending is what people posted with lately: each person once,
	// however many of their posts carry it, so that one account cannot
	// make something trend; by when the post was written, not when it came
	// here, so that a profile's older posts fetched when it is followed do
	// not trend that day; and only posts anybody may see, on profiles
	// anybody may see, so that what trends says nothing of what is kept
	// from view (the user, 2026-10-17). The tag is node_a of its edges.
	query := GetBaseSelectQuery()

	query = query.
		Join("edges AS edge ON edge.node_a_id = node.id").
		Join("nodes AS post ON post.id = edge.node_b_id").
		LeftJoin("nodes AS post_owner ON post_owner.id = post.owner_id").
		Where(sq.Gt{"post.created_at": cutoff}).
		Where("post.access = 'public'").
		Where("post.enabled = 1").
		Where("post.draft = 0").
		Where("post.deleted_at IS NULL").
		Where("(post_owner.id IS NULL OR post_owner.access IS NULL OR post_owner.access = 'public')").
		Where("node.deleted_at IS NULL").
		GroupBy("node.id").
		OrderBy(
			"COUNT(DISTINCT post.created_by) DESC",
			"COUNT(DISTINCT post.id) DESC",
			"node.id DESC",
		)

	// Optional filters
	if req.NodeType != "" {
		query = query.Where(sq.Eq{"node.type": req.NodeType})
	}
	if len(req.EdgeTypes) > 0 {
		query = query.Where(sq.Eq{"edge.type": req.EdgeTypes})
	}

	// Apply access checks if supported by viewer
	if req.AccessCheck {
		query = SetAccessQuery(query, viewer)
	}

	// Limit
	if req.Limit <= 0 {
		req.Limit = constants.DefaultPageSize
	}
	query = query.Limit(req.Limit)

	return FetchNodes(r.db, query)
}

// trendingCutoff is when a trending window begins.
func trendingCutoff(window string) time.Time {
	var cutoff time.Time
	now := time.Now()
	switch window {
	case constants.TimeWindow1h:
		cutoff = now.Add(-1 * time.Hour)
	case constants.TimeWindow6h:
		cutoff = now.Add(-6 * time.Hour)
	case constants.TimeWindow24h:
		cutoff = now.Add(-24 * time.Hour)
	case constants.TimeWindowDaily:
		cutoff = now.AddDate(0, 0, -1)
	case constants.TimeWindowWeekly:
		cutoff = now.AddDate(0, 0, -7)
	case constants.TimeWindowMonthly:
		cutoff = now.AddDate(0, -1, 0)
	case constants.TimeWindowYearly:
		cutoff = now.AddDate(-1, 0, 0)
	default:
		cutoff = now.Add(-24 * time.Hour) // fallback to 24h
	}
	return cutoff
}

// The people who posted with each of some tags since a time, each once, in
// posts Trending counts.
const trendingPeople = `
SELECT edge.node_a_id,
       COUNT(DISTINCT post.created_by)
FROM edges AS edge
JOIN nodes AS post ON post.id = edge.node_b_id
LEFT JOIN nodes AS post_owner ON post_owner.id = post.owner_id
WHERE edge.node_a_id IN ({{ids}})
  AND edge.type IN ({{types}})
  AND post.created_at > ?
  AND post.access = 'public'
  AND post.enabled = 1
  AND post.draft = 0
  AND post.deleted_at IS NULL
  AND (post_owner.id IS NULL OR post_owner.access IS NULL OR post_owner.access = 'public')
GROUP BY edge.node_a_id`

func (r *graphRepo) TrendingPeople(req *graphpb.TrendingRequest, ids []int64) (map[int64]uint64, error) {
	people := map[int64]uint64{}
	if len(ids) == 0 || len(req.EdgeTypes) == 0 {
		return people, nil
	}
	args := []any{}
	for _, id := range ids {
		args = append(args, id)
	}
	for _, each := range req.EdgeTypes {
		args = append(args, each)
	}
	args = append(args, trendingCutoff(req.TimeWindow))
	query := strings.NewReplacer(
		"{{ids}}", placeholders(len(ids)),
		"{{types}}", placeholders(len(req.EdgeTypes)),
	).Replace(trendingPeople)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var count uint64
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		people[id] = count
	}
	return people, rows.Err()
}

// setBlockedLikersQuery leaves out, of the people who like something, those
// its owner or its writer has blocked: a block takes out of sight, for every
// reader, what the blocked person left on what is the blocker's. Their like
// is kept, and comes back if the block is lifted.
func setBlockedLikersQuery(query sq.SelectBuilder, edgeTypes []string, direction string) sq.SelectBuilder {
	if len(edgeTypes) != 1 || edgeTypes[0] != constants.EdgeBaseLikeEntityType {
		return query
	}
	if direction != "" && direction != constants.GraphQueryDirectionInbound {
		return query
	}
	return query.Where(sq.Expr(`NOT EXISTS (
  SELECT 1
  FROM edges AS liker_block
  JOIN nodes AS liked
    ON liked.id = edge.node_b_id
  WHERE liker_block.type = ?
    AND liker_block.node_b_id = node.id
    AND liker_block.node_a_id IN (liked.owner_id, liked.created_by)
)`, constants.EdgeBlockEntityType))
}
