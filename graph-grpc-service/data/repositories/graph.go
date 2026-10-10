package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
)

func NewGraphRepo(db *sql.DB) GraphRepoInterface {
	return &graphRepo{
		db:        db,
		dbTimeout: constants.DBTimeout,
	}
}

type graphRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
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
			query = query.OrderBy("(COALESCE(node.repost_count,0) + COALESCE(node.comment_count,0) + COALESCE(node.vote_up_count,0) + COALESCE(node.subscriber_count,0) + COALESCE(node.follower_count,0)) DESC")
		} else if req.Ordering == constants.OrderingRecent {
			query = query.OrderBy("node.created_at DESC")
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

// Trending returns nodes that have had the most activity in a given time window.
//
// This method:
//   - supports filtering by node type, edge types, and direction
//   - ranks nodes based on recent edge creation activity
//   - performs permission checks based on the viewer
//
// Use this for discovery features, trending topics,
// or highlighting active entities in the graph.
func (r *graphRepo) Trending(req *graphpb.TrendingRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	// ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	// defer cancel()

	// Resolve time window cutoff
	var cutoff time.Time
	now := time.Now()
	switch req.TimeWindow {
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

	query := GetBaseSelectQuery()

	query = query.Join("edges AS edge ON " + r.buildDegreeJoinCondition(req.Direction)).
		Where(sq.Gt{"edge.created_at": cutoff}).
		GroupBy("node.id").
		OrderBy("count(edge.id) DESC")

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
