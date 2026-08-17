package repositories

import (
	"anahita-libs/data/entities"
	"context"
	"database/sql"
	"errors"
	"graph/requests"

	sq "github.com/Masterminds/squirrel"
)

// Read retrieves a single edge by its ID
func (r *edgesRepo) Read(id int64) (*entities.AnahitaEdge, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Select("*").From("edges AS edge").Where(sq.Eq{"edge.id": id})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	row := r.client.QueryRowContext(ctx, sqlQuery, args...)

	item := entities.NewAnahitaEdge()
	var startDate, endDate sql.NullTime
	err = row.Scan(
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
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if startDate.Valid {
		item.StartDate = startDate.Time
	}
	if endDate.Valid {
		item.EndDate = endDate.Time
	}

	return item, nil
}

// ReadByFilters retrieves a single edge based on provided filters
func (r *edgesRepo) ReadByFilters(req *requests.EdgeReadByFiltersRequest) (*entities.AnahitaEdge, error) {
	batchReq := &requests.EdgeBatchReadByFiltersRequest{
		Types:     req.Types,
		NodeAID:   req.NodeAID,
		NodeAType: req.NodeAType,
		NodeBID:   req.NodeBID,
		NodeBType: req.NodeBType,
	}

	edges, err := r.BatchReadByFilters(batchReq)
	if err != nil {
		return nil, err
	}

	if len(edges) == 0 {
		return nil, nil
	}

	return edges[0], nil
}

// BatchRead retrieves multiple edges by their IDs
func (r *edgesRepo) BatchRead(ids []int64) ([]*entities.AnahitaEdge, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if len(ids) == 0 {
		return nil, errors.New("ids are required")
	}

	query := sq.Select("*").From("edges AS edge").Where(sq.Eq{"edge.id": ids})

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

// BatchReadByFilters retrieves multiple edges based on provided filters
func (r *edgesRepo) BatchReadByFilters(req *requests.EdgeBatchReadByFiltersRequest) ([]*entities.AnahitaEdge, error) {
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
