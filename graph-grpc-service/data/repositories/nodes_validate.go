package repositories

import (
	"anahita-libs/data/entities"
	"errors"
)

// validateAdd checks the validity of the node data before adding it to the database.
func (r *nodesRepo) validateAdd(node *entities.AnahitaNode) error {
	if node.Type == "" {
		return errors.New("type is required")
	}

	if node.Service == "" {
		return errors.New("service is required")
	}

	if node.OwnerID > 0 && node.OwnerType == "" {
		return errors.New("owner_type is required when owner_id is set")
	}

	if node.ParentID > 0 && node.ParentType == "" {
		return errors.New("parent_type is required when parent_id is set")
	}

	if node.CreatedBy == 0 {
		return errors.New("created_by is required")
	}

	return nil
}

// validateEdit checks the validity of the node data before editing it in the database.
func (r *nodesRepo) validateEdit(node *entities.AnahitaNode) error {
	if node.ID == 0 {
		return errors.New("id is required")
	}

	if node.Type == "" {
		return errors.New("type is required")
	}

	if node.Service == "" {
		return errors.New("service is required")
	}

	if node.OwnerID > 0 && node.OwnerType == "" {
		return errors.New("owner_type is required when owner_id is set")
	}

	if node.ParentID > 0 && node.ParentType == "" {
		return errors.New("parent_type is required when parent_id is set")
	}

	if node.ModifiedBy == 0 {
		return errors.New("modified_by is required")
	}

	return nil
}
