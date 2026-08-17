package repositories

import (
	"context"
	"errors"
	"graph/requests"

	sq "github.com/Masterminds/squirrel"
)

// Delete removes an edge by its ID
func (r *edgesRepo) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Delete("edges").Where(sq.Eq{"id": id})
	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = r.client.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return err
	}

	return nil
}

// BatchDelete removes multiple edges by their IDs
func (r *edgesRepo) BatchDelete(ids []int64) (uint32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if len(ids) == 0 {
		return 0, errors.New("ids are required")
	}

	query := sq.Delete("edges").Where(sq.Eq{"id": ids})
	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	result, err := r.client.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return uint32(rowsAffected), nil
}

// BatchDeleteByFilters removes multiple edges based on provided filters
func (r *edgesRepo) BatchDeleteByFilters(req *requests.EdgeBatchDeleteByFiltersRequest) (uint32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := sq.Delete("edges")

	if len(req.Types) > 0 {
		query = query.Where(sq.Eq{"type": req.Types})
	}

	if req.NodeAID != 0 {
		query = query.Where(sq.Eq{"node_a_id": req.NodeAID})
	}

	if req.NodeAType != "" {
		query = query.Where(sq.Eq{"node_a_type": req.NodeAType})
	}

	if req.NodeBID != 0 {
		query = query.Where(sq.Eq{"node_b_id": req.NodeBID})
	}

	if req.NodeBType != "" {
		query = query.Where(sq.Eq{"node_b_type": req.NodeBType})
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	result, err := r.client.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return 0, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return uint32(rowsAffected), nil
}

// BatchDeleteByNodeIDs removes multiple edges based on provided node IDs
func (r *edgesRepo) BatchDeleteByNodeIDs(nodeIDs []int64) (uint32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if len(nodeIDs) == 0 {
		return 0, errors.New("node IDs are required")
	}

	query := sq.Delete("edges").Where(sq.Or{
		sq.Eq{"node_a_id": nodeIDs},
		sq.Eq{"node_b_id": nodeIDs},
	})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	result, err := r.client.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return uint32(rowsAffected), nil
}
