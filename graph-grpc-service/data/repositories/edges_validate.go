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

	// Nothing else: an edit changes neither end of an edge, its type nor who
	// made it (edges_edit.go). It used to call itself here, and the first
	// edge ever edited, a tag approved (roadmap row 103), took the service
	// down.
	return nil
}
