package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"
	"database/sql"
	"graph/requests"
	"time"

	sq "github.com/Masterminds/squirrel"
)

func NewEdgesRepo(db *sql.DB) EdgesRepoInterface {
	return &edgesRepo{
		client:    db,
		dbTimeout: constants.DBTimeout,
	}
}

type edgesRepo struct {
	client    *sql.DB
	dbTimeout time.Duration
}

// Browse retrieves a list of edges based on the provided filters
func (r *edgesRepo) Browse(req *requests.EdgeBrowseRequest) ([]*entities.AnahitaEdge, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Select("*").From("edges AS edge")

	if len(req.Types) > 0 {
		query = query.Where(sq.Eq{"edge.type": req.Types})
	}

	if req.NodeAID != 0 {
		query = query.Where(sq.Eq{"edge.node_a_id": req.NodeAID})
	}

	if req.NodeAType != "" {
		query = query.Where(sq.Eq{"edge.node_a_type": req.NodeAType})
	}

	if req.NodeBID != 0 {
		query = query.Where(sq.Eq{"edge.node_b_id": req.NodeBID})
	}

	if req.NodeBType != "" {
		query = query.Where(sq.Eq{"edge.node_b_type": req.NodeBType})
	}

	if req.Limit != 0 {
		query = query.Limit(uint64(req.Limit))
	}

	if req.Offset != 0 {
		query = query.Offset(uint64(req.Offset))
	}

	if req.Ordering != "" {
		query = query.OrderBy(req.Ordering)
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.client.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*entities.AnahitaEdge
	for rows.Next() {
		item := entities.NewAnahitaEdge()
		var startDate, endDate sql.NullTime
		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.NodeAID,
			&item.NodeAType,
			&item.NodeBID,
			&item.NodeBType,
			&item.Meta,
			&item.CreatedOn,
			&item.CreatedBy,
			&item.ModifiedOn,
			&item.ModifiedBy,
			&item.Ordering,
			&startDate,
			&endDate,
		)
		if err != nil {
			return nil, err
		}

		if startDate.Valid {
			item.StartDate = startDate.Time
		}
		if endDate.Valid {
			item.EndDate = endDate.Time
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// Count returns the number of edges matching the provided filters
func (r *edgesRepo) Count(req *requests.EdgeCountRequest) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Select("COUNT(*)").From("edges AS edge")

	if len(req.Types) > 0 {
		query = query.Where(sq.Eq{"edge.type": req.Types})
	}

	if req.NodeAID != 0 {
		query = query.Where(sq.Eq{"edge.node_a_id": req.NodeAID})
	}

	if req.NodeAType != "" {
		query = query.Where(sq.Eq{"edge.node_a_type": req.NodeAType})
	}

	if req.NodeBID != 0 {
		query = query.Where(sq.Eq{"edge.node_b_id": req.NodeBID})
	}

	if req.NodeBType != "" {
		query = query.Where(sq.Eq{"edge.node_b_type": req.NodeBType})
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	row := r.client.QueryRowContext(ctx, sqlQuery, args...)

	var count uint64
	err = row.Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Exists checks if an edge exists based on the provided criteria
func (r *edgesRepo) Exists(edge *entities.AnahitaEdge) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Select("1").From("edges AS edge").Limit(1)

	if edge.ID != 0 {
		query = query.Where(sq.Eq{"edge.id": edge.ID})
	}

	if edge.Type != "" {
		query = query.Where(sq.Eq{"edge.type": edge.Type})
	}

	if edge.NodeAID != 0 {
		query = query.Where(sq.Eq{"edge.node_a_id": edge.NodeAID})
	}

	if edge.NodeAType != "" {
		query = query.Where(sq.Eq{"edge.node_a_type": edge.NodeAType})
	}

	if edge.NodeBID != 0 {
		query = query.Where(sq.Eq{"edge.node_b_id": edge.NodeBID})
	}

	if edge.NodeBType != "" {
		query = query.Where(sq.Eq{"edge.node_b_type": edge.NodeBType})
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return false, err
	}

	var exists int
	err = r.client.QueryRowContext(ctx, sqlQuery, args...).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
