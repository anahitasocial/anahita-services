package repositories

import (
	"anahita-libs/data/entities"
	"errors"
)

// validateAdd validates the edge before adding
func (r *edgesRepo) validateAdd(edge *entities.AnahitaEdge) error {
	if edge.Type == "" {
		return errors.New("type is required")
	}

	if edge.NodeAID == 0 {
		return errors.New("node_a_id is required")
	}

	if edge.NodeAType == "" {
		return errors.New("node_a_type is required")
	}

	if edge.NodeBID == 0 {
		return errors.New("node_b_id is required")
	}

	if edge.NodeBType == "" {
		return errors.New("node_b_type is required")
	}

	if edge.CreatedBy == 0 {
		return errors.New("created_by is required")
	}

	return nil
}

// validateEdit validates the edge before editing
func (r *edgesRepo) validateEdit(edge *entities.AnahitaEdge) error {
	if edge.ID == 0 {
		return errors.New("id is required")
	}

	if edge.ModifiedBy == 0 {
		return errors.New("modified_by is required")
	}

	if err := r.validateEdit(edge); err != nil {
		return err
	}

	return nil
}
