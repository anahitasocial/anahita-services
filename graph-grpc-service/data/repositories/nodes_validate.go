package repositories

import (
	"anahita-libs/constants"
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

	// A person is the exception: it has no creator.
	//
	// Every other node is made BY somebody — a group by its founder, a photo by
	// its author — and recording who is worth requiring. A person node is the
	// somebody. At signup there is no other party and no id to name even in
	// principle, since the id in question is the one this insert is about to
	// assign.
	//
	// The requirement never applied to that path before: auth-service wrote
	// `nodes` with a direct INSERT, which reached no validator and left
	// created_by NULL. Routing signup through ActorService is what surfaced it.
	if node.CreatedBy == 0 && node.Type != constants.NodeActorPersonEntityType {
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
