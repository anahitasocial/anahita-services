package repositories

import (
	"anahita-libs/data/entities"
	"anahita-libs/feeds"
	"anahita-libs/proto/graphpb"
	"fmt"
	"time"

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
	}
}

// SearchRepo is the Search repository
type SearchRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
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

func (repo *SearchRepo) GetRelevantSortQuery() string {
	return "(COALESCE(node.comment_count,0) + COALESCE(node.vote_up_count,0) + COALESCE(node.subscriber_count,0) + COALESCE(node.follower_count,0)) DESC"
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
		query = query.Where(sq.Or{
			sq.Like{"node.name": "%" + keywords + "%"},
			sq.Like{"node.alias": "%" + keywords + "%"},
			sq.Like{"node.body": "%" + keywords + "%"},
		})
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

// Browse returns a list of Searchs
func (repo *SearchRepo) Browse(req *graphpb.SearchBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	query := repo.getBaseSelectQuery(req)
	query = repo.getScopeQuery(query, req.Scope)
	query = SetSearchAccessQuery(query, viewer)
	query = SetLifecycleQuery(query, viewer)
	query = SetArchiveSearchQuery(query, viewer)

	// Strip LIKE metacharacters, as the node browse does. Without this a
	// search for "%" matches every row. Count and Browse must sanitise
	// identically or the total describes a different set than the rows.
	if keywords := sanitizeSearchString(req.Keywords); keywords != "" {
		query = query.Where(sq.Or{
			sq.Like{"node.name": "%" + keywords + "%"},
			sq.Like{"node.alias": "%" + keywords + "%"},
			sq.Like{"node.body": "%" + keywords + "%"},
		})
	}

	if req.CoordLat != 0.0 && req.CoordLng != 0.0 {
		calculateDistance := repo.calculateDistance(req.CoordLat, req.CoordLng)
		// In metres, as in Count. Multiplied again here, a search within
		// 25 km listed what was 25,000 km away.
		query = query.Where(fmt.Sprintf("%s <= %d", calculateDistance, req.CoordRadius))
	}

	if req.Ordering == "relevant" {
		relevantSortQuery := repo.GetRelevantSortQuery()
		query = query.OrderBy(relevantSortQuery)
	} else {
		query = query.OrderBy("node.created_at DESC")
	}

	if req.CoordLat != 0.0 && req.CoordLng != 0.0 {
		distanceCalculation := repo.calculateDistance(req.CoordLat, req.CoordLng)
		query = query.OrderBy(fmt.Sprintf("%s ASC", distanceCalculation))
	}

	query = query.Limit(req.Limit).Offset(req.Offset)

	return FetchNodes(repo.db, query)
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
