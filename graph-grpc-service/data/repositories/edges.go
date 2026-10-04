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

	query := sq.Select(edgeColumns()...).From("edges AS edge")

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

	if column := safeOrderColumn(req.Ordering); column != "" {
		query = query.OrderBy(column)
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
		var startsAt, endsAt sql.NullTime
		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.NodeAID,
			&item.NodeAType,
			&item.NodeBID,
			&item.NodeBType,
			&item.Meta,
			&item.CreatedAt,
			&item.CreatedBy,
			&item.UpdatedAt,
			&item.ModifiedBy,
			&item.Ordering,
			&startsAt,
			&endsAt,
		)
		if err != nil {
			return nil, err
		}

		if startsAt.Valid {
			item.StartsAt = startsAt.Time
		}
		if endsAt.Valid {
			item.EndsAt = endsAt.Time
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

// edgeColumns is the select list every edge read uses, in the order the scans
// expect.
//
// Named rather than `SELECT *` for two reasons. The scans are positional, so a
// column added to `edges` silently shifts every one of them; and `meta`,
// `created_by` and `modified_by` are nullable, which a `SELECT *` hands to the
// scan as NULL — "converting NULL to string is unsupported" — failing the whole
// read. Every mention edge in the reference database has a NULL meta.
func edgeColumns() []string {
	return []string{
		"edge.id",
		"edge.type",
		"edge.node_a_id",
		"edge.node_a_type",
		"edge.node_b_id",
		"edge.node_b_type",
		"COALESCE(edge.meta, '') AS meta",
		"edge.created_at",
		"COALESCE(edge.created_by, 0) AS created_by",
		"edge.updated_at",
		"COALESCE(edge.modified_by, 0) AS modified_by",
		"COALESCE(edge.ordering, 0) AS ordering",
		"edge.starts_at",
		"edge.ends_at",
	}
}
