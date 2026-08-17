package repositories

import (
	"anahita-libs/data/entities"
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
)

// Add creates a new edge
func (r *edgesRepo) Add(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if err := r.validateAdd(edge); err != nil {
		return nil, err
	}

	var startDate, endDate any
	if !edge.StartDate.IsZero() {
		startDate = edge.StartDate
	}
	if !edge.EndDate.IsZero() {
		endDate = edge.EndDate
	}

	query := sq.Insert("edges").Columns(
		"type",
		"node_a_id",
		"node_a_type",
		"node_b_id",
		"node_b_type",
		"meta",
		"created_on",
		"created_by",
		"modified_on",
		"modified_by",
		"ordering",
		"start_date",
		"end_date",
	).Values(
		edge.Type,
		edge.NodeAID,
		edge.NodeAType,
		edge.NodeBID,
		edge.NodeBType,
		edge.Meta,
		time.Now(),
		edge.CreatedBy,
		time.Now(),
		edge.ModifiedBy,
		edge.Ordering,
		startDate,
		endDate,
	)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	result, err := r.client.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.Read(id)
}

// BatchAdd creates multiple edges
func (r *edgesRepo) BatchAdd(edges []*entities.AnahitaEdge) ([]*entities.AnahitaEdge, error) {
	if len(edges) == 0 {
		return nil, errors.New("edges are required")
	}

	for _, edge := range edges {
		addedEdge, err := r.Add(edge)
		if err != nil {
			return nil, err
		}
		edges = append(edges, addedEdge)
	}

	return edges, nil
}
