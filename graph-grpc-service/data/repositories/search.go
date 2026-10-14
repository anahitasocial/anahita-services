package repositories

import (
	"anahita-libs/data/entities"
	"anahita-libs/feeds"
	"anahita-libs/proto/graphpb"
	"fmt"
	"regexp"
	"strings"
	"time"

	"graph/ranking"

	"anahita-libs/constants"
	"context"
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

const earthRadius = 6371000 // metres: distances, and the radius asked for, are in metres

type SearchRepoInterface interface {
	// Count returns the number of Searchs
	Count(req *graphpb.SearchCountRequest, viewer *entities.Viewer) (uint64, error)

	// Browse returns a list of Searchs
	Browse(req *graphpb.SearchBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)
}

// NewSearchRepo creates a new Search repository
func NewSearchRepo(db *sql.DB) SearchRepoInterface {
	return &SearchRepo{
		db:        db,
		dbTimeout: constants.DBTimeout,
		ranking:   ranking.FromEnv(),
	}
}

// SearchRepo is the Search repository
type SearchRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
	// The weights "Most relevant" is ordered by. See package ranking.
	ranking *ranking.Source
}

// getBaseSelectQuery returns the base select query
func (repo *SearchRepo) getBaseSelectQuery(req *graphpb.SearchBrowseRequest) sq.SelectBuilder {
	query := GetBaseSelectQuery()

	if req.CoordLat != 0.0 && req.CoordLng != 0.0 {
		// Use INNER JOIN since we only want nodes that have a location
		query = query.Join(
			fmt.Sprintf("edges AS location_edge ON location_edge.node_b_id = node.id AND location_edge.type = '%s'", constants.EdgeTagLocationType),
		).Join(
			"nodes AS location ON location_edge.node_a_id = location.id",
		)
	}

	return query
}

func (repo *SearchRepo) calculateDistance(lat float32, lng float32) string {
	return fmt.Sprintf(
		"CEIL((ACOS(SIN(%.3f * PI()/180) * SIN(location.geo_latitude * PI()/180) + COS(%.3f * PI()/180) * COS(location.geo_latitude * PI()/180) * COS((%.3f * PI()/180) - (location.geo_longitude * PI()/180))) * %d))",
		lat, lat, lng, earthRadius,
	)
}

// handleOf is what was searched for as a handle may be written, without
// the `@` or `#` people put before one.
func handleOf(keywords string) string {
	return strings.ToLower(strings.TrimLeft(keywords, "@#"))
}

// wordsMatch is the condition that a node has what was searched for: in
// its name, its handle or its body. The handle is matched without the `@`
// somebody typed before it, so that `@bmann.ca` finds bmann.ca.
func wordsMatch(keywords string) sq.Or {
	return sq.Or{
		sq.Like{"node.name": "%" + keywords + "%"},
		sq.Like{"node.alias": "%" + handleOf(keywords) + "%"},
		sq.Like{"node.body": "%" + keywords + "%"},
	}
}

// How well a node matches, as points for where the words are. The first
// that holds is the one counted.
const relevantMatch = `(CASE
    WHEN LOWER(node.alias) = ? THEN ?
    WHEN LOWER(node.name) = ? THEN ?
    WHEN node.name LIKE ? THEN ?
    WHEN node.name LIKE ? THEN ?
    WHEN node.name LIKE ? THEN ?
    WHEN node.alias LIKE ? THEN ?
    WHEN EXISTS (
        SELECT 1
        FROM edges AS tagged
        JOIN nodes AS hashtag ON hashtag.id = tagged.node_a_id
        WHERE tagged.node_b_id = node.id
          AND tagged.type = ?
          AND LOWER(hashtag.name) = ?
    ) THEN ?
    WHEN REGEXP_LIKE(COALESCE(node.body, ''), ?, 'i') THEN ?
    ELSE ?
END)`

// How useful people have found a node: its likes, saves, replies, reposts,
// quotes and followers, each counted by when it happened (see interest),
// each by a lookup an index answers (replies and quotes apart, since one
// condition on either would read every node for every result; and the
// index on quotes named, since while nearly no post quotes another the
// database judges it not worth using and reads every node instead),
// on a logarithm so that a thousand comes first and not a hundred times
// first. Then a bonus for what the searcher follows.
const relevantStanding = `(
    ? * LN(1
        + ? * (
            SELECT COALESCE(SUM(%[1]s), 0)
            FROM edges AS liked
            WHERE liked.node_b_id = node.id
              AND liked.type = ?
        )
        + ? * (
            SELECT COALESCE(SUM(%[2]s), 0)
            FROM edges AS saved
            WHERE saved.node_b_id = node.id
              AND saved.type = ?
        )
        + ? * (
            (
                SELECT COALESCE(SUM(%[3]s), 0)
                FROM nodes AS answer
                WHERE answer.parent_id = node.id
            ) + (
                SELECT COALESCE(SUM(%[6]s), 0)
                FROM nodes AS quoting FORCE INDEX (quote_id)
                WHERE quoting.quote_id = node.id
            )
        )
        + ? * (
            SELECT COALESCE(SUM(%[4]s), 0)
            FROM edges AS followed
            WHERE followed.node_b_id = node.id
              AND followed.type = ?
        )
    )
    + %[5]s
)`

// What the searcher follows, or what is on a profile they follow.
const followedByViewer = `(CASE WHEN EXISTS (
        SELECT 1
        FROM edges AS mine
        WHERE mine.type = ?
          AND mine.node_a_id = ?
          AND mine.node_b_id IN (node.id, node.owner_id)
    ) THEN ? ELSE 0 END)`

// interest is how much one sign of interest counts, by when it happened:
// all of it with no half-life, half of it after each half-life.
func interest(column string, halfLifeDays float64) string {
	if halfLifeDays <= 0 {
		return "1"
	}
	return fmt.Sprintf("POW(0.5, GREATEST(TIMESTAMPDIFF(HOUR, %s, UTC_TIMESTAMP()), 0) / %.4f)", column, halfLifeDays*24)
}

// relevance is the two scores a node is ranked by, as columns: how well
// it matches, and its standing, as the operator's weights say.
//
// They are worked out once for each node, as columns, and the order is by
// those columns. Ordered by the expressions themselves, the database ran
// the lookups of the standing again and again as it sorted, and a broad
// word took ten seconds.
func relevance(keywords string, weights ranking.Weights, viewer *entities.Viewer) (match, standing sq.Sqlizer) {
	lower := strings.ToLower(keywords)
	m := weights.Match

	match = sq.Expr(relevantMatch+" AS match_points",
		handleOf(keywords), m.Handle,
		lower, m.Name,
		keywords+"%", m.NameStarts,
		"% "+keywords+"%", m.WordStarts,
		"%"+keywords+"%", m.NameContains,
		// Inside the handle (`decafcoffeeclub`) is as good as inside the
		// name.
		"%"+handleOf(keywords)+"%", m.NameContains,
		constants.EdgeTagHashtagType, strings.TrimLeft(lower, "#"), m.Hashtag,
		`\b`+regexp.QuoteMeta(keywords)+`\b`, m.BodyWord,
		m.BodyContains,
	)

	following := "0"
	args := []any{
		weights.Useful.Scale,
		weights.Useful.Like, constants.EdgeBaseLikeEntityType,
		weights.Useful.Save, constants.EdgeBaseBookmarkEntityType,
		weights.Useful.Reply,
		weights.Useful.Follow, constants.EdgeFollowEntityType,
	}
	if viewer != nil && viewer.ID > 0 {
		following = followedByViewer
		args = append(args, constants.EdgeFollowEntityType, viewer.ID, weights.Following)
	}
	standing = sq.Expr(fmt.Sprintf(relevantStanding,
		interest("liked.created_at", weights.HalfLifeDays),
		interest("saved.created_at", weights.HalfLifeDays),
		interest("answer.created_at", weights.HalfLifeDays),
		interest("followed.created_at", weights.HalfLifeDays),
		following,
		interest("quoting.created_at", weights.HalfLifeDays),
	)+" AS standing", args...)
	return match, standing
}

// Count returns the number of Searchs
func (repo *SearchRepo) Count(req *graphpb.SearchCountRequest, viewer *entities.Viewer) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	query := sq.Select("COUNT(*)").From("nodes AS node")
	query = query.LeftJoin("nodes AS owner ON owner.id = node.owner_id")

	if req.CoordLat != 0.0 && req.CoordLng != 0.0 {
		// Use INNER JOIN since we only want nodes that have a location
		query = query.Join(
			fmt.Sprintf("edges AS location_edge ON location_edge.node_b_id = node.id AND location_edge.type = '%s'", constants.EdgeTagLocationType),
		).Join(
			"nodes AS location ON location_edge.node_a_id = location.id",
		)
	}

	query = repo.getScopeQuery(query, req.Scope)
	query = SetSearchAccessQuery(query, viewer)
	query = SetLifecycleQuery(query, viewer)
	query = SetArchiveSearchQuery(query, viewer)

	// Strip LIKE metacharacters, as the node browse does. Without this a
	// search for "%" matches every row. Count and Browse must sanitise
	// identically or the total describes a different set than the rows.
	if keywords := sanitizeSearchString(req.Keywords); keywords != "" {
		query = query.Where(wordsMatch(keywords))
	}

	if req.CoordLat != 0.0 && req.CoordLng != 0.0 {
		calculateDistance := repo.calculateDistance(req.CoordLat, req.CoordLng)
		query = query.Where(fmt.Sprintf("%s <= %d", calculateDistance, req.CoordRadius))
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var count uint64
	err = repo.db.QueryRowContext(ctx, sql, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Browse returns a list of Searchs.
//
// Ordered by date, the database stops after the first page. Ordered by
// relevance it must score every match before it knows the first page, and
// building every match's whole row (the author, the owner, the parent and
// what else is read with a node) only to sort them took longer than the
// time a query is given. So relevance is two steps: the ids of the page,
// ranked on the node and its owner alone, then the whole rows of those.
func (repo *SearchRepo) Browse(req *graphpb.SearchBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	keywords := sanitizeSearchString(req.Keywords)
	if req.Ordering == "relevant" && keywords != "" {
		return repo.browseByRelevance(req, keywords, viewer)
	}

	query := repo.filtered(repo.getBaseSelectQuery(req), req, viewer)
	query = query.OrderBy("node.created_at DESC")
	query = repo.nearestThenID(query, req)
	query = query.Limit(req.Limit).Offset(req.Offset)

	return FetchNodes(repo.db, query)
}

// filtered is what is found at all: the tab, what the viewer may see, the
// words and the distance.
func (repo *SearchRepo) filtered(query sq.SelectBuilder, req *graphpb.SearchBrowseRequest, viewer *entities.Viewer) sq.SelectBuilder {
	query = repo.getScopeQuery(query, req.Scope)
	query = SetSearchAccessQuery(query, viewer)
	query = SetLifecycleQuery(query, viewer)
	query = SetArchiveSearchQuery(query, viewer)

	// Strip LIKE metacharacters, as the node browse does. Without this a
	// search for "%" matches every row. Count and Browse must sanitise
	// identically or the total describes a different set than the rows.
	if keywords := sanitizeSearchString(req.Keywords); keywords != "" {
		query = query.Where(wordsMatch(keywords))
	}

	if req.CoordLat != 0.0 && req.CoordLng != 0.0 {
		calculateDistance := repo.calculateDistance(req.CoordLat, req.CoordLng)
		// In metres, as in Count. Multiplied again here, a search within
		// 25 km listed what was 25,000 km away.
		query = query.Where(fmt.Sprintf("%s <= %d", calculateDistance, req.CoordRadius))
	}
	return query
}

// nearestThenID ends an order: the nearest first when a distance was
// chosen, then the id, so that two of the same standing keep their places
// from one page to the next.
func (repo *SearchRepo) nearestThenID(query sq.SelectBuilder, req *graphpb.SearchBrowseRequest) sq.SelectBuilder {
	if req.CoordLat != 0.0 && req.CoordLng != 0.0 {
		distanceCalculation := repo.calculateDistance(req.CoordLat, req.CoordLng)
		query = query.OrderBy(fmt.Sprintf("%s ASC", distanceCalculation))
	}
	return query.OrderBy("node.id DESC")
}

func (repo *SearchRepo) browseByRelevance(req *graphpb.SearchBrowseRequest, keywords string, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	// The page's ids. What may be seen is decided on the node and its
	// owner, which is all this reads.
	ranked := sq.Select("node.id").From("nodes AS node").
		LeftJoin("nodes AS owner ON node.owner_id = owner.id")
	if req.CoordLat != 0.0 && req.CoordLng != 0.0 {
		ranked = ranked.Join(
			fmt.Sprintf("edges AS location_edge ON location_edge.node_b_id = node.id AND location_edge.type = '%s'", constants.EdgeTagLocationType),
		).Join(
			"nodes AS location ON location_edge.node_a_id = location.id",
		)
	}
	match, standing := relevance(keywords, repo.ranking.Weights(), viewer)
	ranked = ranked.Column(match).Column(standing)
	// "People and groups I follow first": what they follow, or what is on
	// a profile they follow, before everything else, however it matches.
	followedFirst := req.FollowedFirst && viewer != nil && viewer.ID > 0
	if followedFirst {
		ranked = ranked.Column(sq.Expr(followedByViewer+" AS followed",
			constants.EdgeFollowEntityType, viewer.ID, 1))
	}
	ranked = repo.filtered(ranked, req, viewer)
	if followedFirst {
		ranked = ranked.OrderBy("followed DESC")
	}
	ranked = ranked.OrderBy("match_points DESC")
	// "Prefer recent": among results that match alike, the newest first,
	// and only then by standing.
	if req.RecentFirst {
		ranked = ranked.OrderBy("node.created_at DESC")
	}
	ranked = ranked.OrderBy("standing DESC")
	ranked = repo.nearestThenID(ranked, req)
	ranked = ranked.Limit(req.Limit).Offset(req.Offset)

	ids, err := repo.idsOf(ranked)
	if err != nil || len(ids) == 0 {
		return nil, err
	}

	// Their whole rows, put back in the order they were ranked in.
	nodes, err := FetchNodes(repo.db, GetBaseSelectQuery().Where(sq.Eq{"node.id": ids}))
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*entities.AnahitaNode, len(nodes))
	for _, node := range nodes {
		byID[node.ID] = node
	}
	ordered := make([]*entities.AnahitaNode, 0, len(ids))
	for _, id := range ids {
		if node, ok := byID[id]; ok {
			ordered = append(ordered, node)
		}
	}
	return ordered, nil
}

// idsOf reads the first column of each row, the id; the scores after it
// are only there to order by.
func (repo *SearchRepo) idsOf(query sq.SelectBuilder) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := repo.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		rest := make([]any, len(columns)-1)
		for i := range rest {
			rest[i] = new(sql.RawBytes)
		}
		if err := rows.Scan(append([]any{&id}, rest...)...); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (repo *SearchRepo) getScopeQuery(query sq.SelectBuilder, scope string) sq.SelectBuilder {
	actorCondition := sq.Like{"node.type": constants.NodeActorPrefixEntityType + "%"}
	mediumCondition := sq.Like{"node.type": constants.NodeMediumPrefixEntityType + "%"}

	// A custom feed somebody has shared. One kept to its owner is not
	// found, by anybody: its owner has it under their own feeds.
	feedCondition := sq.And{
		sq.Eq{"node.type": feeds.NodeType},
		sq.NotEq{"node.access": constants.AnahitaAccessMyself},
	}

	switch scope {
	case constants.SearchScopeActor:
		return query.Where(actorCondition)
	case constants.SearchScopeMedium:
		return query.Where(mediumCondition)
	case constants.SearchScopeFeed:
		return query.Where(feedCondition)
	default:
		return query.Where(sq.Or{actorCondition, mediumCondition, feedCondition})
	}
}
