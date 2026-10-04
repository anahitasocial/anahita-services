package repositories

import (
	"anahita-libs/data/entities"
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
)

// Edit updates an existing edge; only meta, updated_at, modified_by, ordering, starts_at, ends_at are editable
func (r *edgesRepo) Edit(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if err := r.validateEdit(edge); err != nil {
		return nil, err
	}

	query := sq.Update("edges").
		Set("updated_at", time.Now()).
		Set("modified_by", edge.ModifiedBy).
		Where(sq.Eq{"id": edge.ID})

	if edge.Meta != "" {
		query = query.Set("meta", edge.Meta)
	}

	if edge.Ordering != 0 {
		query = query.Set("ordering", edge.Ordering)
	}

	if !edge.StartsAt.IsZero() {
		query = query.Set("starts_at", edge.StartsAt)
	}

	if !edge.EndsAt.IsZero() {
		query = query.Set("ends_at", edge.EndsAt)
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	_, err = r.client.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}

	return r.Read(edge.ID)
}

// BatchEdit updates multiple edges
func (r *edgesRepo) BatchEdit(edges []*entities.AnahitaEdge) ([]*entities.AnahitaEdge, error) {
	if len(edges) == 0 {
		return nil, errors.New("edges are required")
	}

	for _, edge := range edges {
		editedEdge, err := r.Edit(edge)
		if err != nil {
			return nil, err
		}
		edges = append(edges, editedEdge)
	}

	return edges, nil
}
