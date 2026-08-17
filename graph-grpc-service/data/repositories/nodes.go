package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"
	"database/sql"
	"fmt"
	"graph/requests"
	"time"

	sq "github.com/Masterminds/squirrel"
)

func NewNodesRepo(db *sql.DB) NodesRepoInterface {
	return &nodesRepo{
		db:        db,
		dbTimeout: constants.DBTimeout,
		edgesRepo: NewEdgesRepo(db).(*edgesRepo),
	}
}

type nodesRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
	edgesRepo *edgesRepo
}

// Browse retrieves a list of nodes based on the provided browse request and viewer context.
func (r *nodesRepo) Browse(req *requests.NodeBrowseRequest) ([]*entities.AnahitaNode, error) {
	query := GetBaseSelectQuery()

	if len(req.Types) > 0 {
		query = query.Where(sq.Eq{"node.type": req.Types})
	}

	if len(req.OwnerIDs) > 0 {
		query = query.Where(sq.Eq{"node.owner_id": req.OwnerIDs})
	}

	if len(req.ParentIDs) > 0 {
		query = query.Where(sq.Eq{"node.parent_id": req.ParentIDs})
	}

	req.Search = sanitizeSearchString(req.Search)
	if req.Search != "" {
		likePattern := "%" + req.Search + "%"
		query = query.Where(sq.Or{
			sq.Like{"node.name": likePattern},
			sq.Like{"node.body": likePattern},
			sq.Like{"owner.name": likePattern},
		})
	}

	// Nearby filtering - only include nodes with valid geo coordinates
	if req.NearbyLatitude != 0 && req.NearbyLongitude != 0 {
		// Filter to only nodes with geo coordinates
		query = query.Where("node.geo_latitude IS NOT NULL")
		query = query.Where("node.geo_longitude IS NOT NULL")

		// Optional: add radius filter if you have a NearbyRadius field
		if req.NearbyRadius > 0 {
			distanceCalc := r.calculateDistance(req.NearbyLatitude, req.NearbyLongitude)
			query = query.Where(fmt.Sprintf("%s <= ?", distanceCalc), req.NearbyRadius) // Convert km to meters if your radius is in km
		}

		// Order by distance (closest first)
		distanceCalc := r.calculateDistance(req.NearbyLatitude, req.NearbyLongitude)
		query = query.OrderBy(fmt.Sprintf("%s ASC", distanceCalc))
	}

	query = query.Where(sq.Eq{"node.enabled": req.Enabled})

	if req.AccessCheck {
		// query = SetAccessQuery(query, req.Viewer)
	}

	if req.Limit == 0 {
		req.Limit = constants.DefaultPageSize
	} else if req.Limit > constants.MaxPageSize {
		req.Limit = constants.MaxPageSize
	}

	if req.Ordering != "" && req.NearbyLatitude == 0 && req.NearbyLongitude == 0 {
		if req.Ordering == constants.OrderingPopular {
			query = query.OrderBy("(COALESCE(node.repost_count,0) + COALESCE(node.comment_count,0) + COALESCE(node.vote_up_count,0) + COALESCE(node.subscriber_count,0) + COALESCE(node.follower_count,0)) DESC")
		} else if req.Ordering == constants.OrderingRecent {
			query = query.OrderBy("node.created_on DESC")
		} else if req.OrderingDir == constants.DirAsc {
			query = query.OrderBy(req.Ordering + " ASC")
		} else {
			query = query.OrderBy(req.Ordering + " DESC")
		}
	}

	query = query.Limit(req.Limit).Offset(req.Offset)

	return FetchNodes(r.db, query)
}

// Count returns the total number of nodes matching the provided browse request and viewer context.
func (r *nodesRepo) Count(req *requests.NodeCountRequest) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Select("COUNT(*)").From(
		"nodes AS node",
	).LeftJoin(
		"nodes AS owner ON node.owner_id = owner.id",
	)

	if len(req.Types) > 0 {
		query = query.Where(sq.Eq{"node.type": req.Types})
	}

	if len(req.OwnerIDs) > 0 {
		query = query.Where(sq.Eq{"node.owner_id": req.OwnerIDs})
	}

	if len(req.ParentIDs) > 0 {
		query = query.Where(sq.Eq{"node.parent_id": req.ParentIDs})
	}

	req.Search = sanitizeSearchString(req.Search)
	if req.Search != "" {
		likePattern := "%" + req.Search + "%"
		query = query.Where(sq.Or{
			sq.Like{"node.name": likePattern},
			sq.Like{"node.body": likePattern},
			sq.Like{"owner.name": likePattern},
		})
	}

	// Nearby filtering - only include nodes with valid geo coordinates
	if req.NearbyLatitude != 0 && req.NearbyLongitude != 0 {
		// Filter to only nodes with geo coordinates
		query = query.Where("node.geo_latitude IS NOT NULL")
		query = query.Where("node.geo_longitude IS NOT NULL")

		// Optional: add radius filter if you have a NearbyRadius field
		if req.NearbyRadius > 0 {
			distanceCalc := r.calculateDistance(req.NearbyLatitude, req.NearbyLongitude)
			query = query.Where(fmt.Sprintf("%s <= ?", distanceCalc), req.NearbyRadius) // Convert km to meters if your radius is in km
		}
	}

	query = query.Where(sq.Eq{"node.enabled": req.Enabled})

	if req.AccessCheck {
		// query = SetAccessQuery(query, req.Viewer)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var count uint64
	err = r.db.QueryRowContext(ctx, sql, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// BrowseMutualConnections retrieves mutual connections between two nodes based on the provided request and viewer context.
func (r *nodesRepo) BrowseMutualConnections(req *requests.NodeBrowseMutualConnectionsRequest) ([]*entities.AnahitaNode, error) {
	// Implementation goes here
	return nil, nil
}

// CountConnections returns the total number of connections for a node based on the provided request and viewer context.
func (r *nodesRepo) CountConnections(req *requests.NodeCountConnectionsRequest) (uint64, error) {
	// Implementation goes here
	return 0, nil
}

// calculateDistance returns SQL for Haversine distance calculation in km
func (repo *nodesRepo) calculateDistance(lat float64, lng float64) string {
	return fmt.Sprintf(
		"CEIL((ACOS(SIN(%.6f * PI()/180) * SIN(node.geo_latitude * PI()/180) + COS(%.6f * PI()/180) * COS(node.geo_latitude * PI()/180) * COS((%.6f * PI()/180) - (node.geo_longitude * PI()/180))) * %d))",
		lat, lat, lng, earthRadius,
	)
}
