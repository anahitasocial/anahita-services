package repositories

import (
	"context"
	"errors"
	"graph/requests"

	sq "github.com/Masterminds/squirrel"
)

// ErrEdgeDeleteNotNarrowed is returned by BatchDeleteByFilters when a request
// carries no node id. See the comment there: without one the statement is
// either unbounded or bounded only by edge type, and both are caller bugs.
var ErrEdgeDeleteNotNarrowed = errors.New(
	"edges: refusing a batch delete with no node id — it would remove every matching edge in the system")

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

	// REFUSE A DELETE THAT IS NOT NARROWED TO A NODE.
	//
	// Every clause below is conditional, so a request with all fields at their
	// zero value produced `DELETE FROM edges` with no WHERE at all — every
	// follow, block, group-admin edge, subscription and vote in the system,
	// removed in one statement that returns a row count and no error.
	//
	// A type-only filter is refused for the same reason at smaller scale: it
	// deletes every edge of that type fleet-wide, which is what a caller gets
	// when a node lookup returns 0 and the id clauses silently drop out. That
	// is a bug in the caller every time, never an intention.
	//
	// The guard lives HERE rather than in the handler because the handler is
	// not the only caller — subscriptions and likes reach this directly — and a
	// check at the boundary would leave the internal paths unprotected.
	//
	// If a fleet-wide purge of one edge type is ever genuinely wanted, it
	// deserves its own named method rather than arriving as the fallback
	// behaviour of a filter that happened to be empty.
	if req.NodeAID == 0 && req.NodeBID == 0 {
		return 0, ErrEdgeDeleteNotNarrowed
	}

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
