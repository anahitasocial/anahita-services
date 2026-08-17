package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
)

func (repo *RepostsRepo) validateAdd(node *entities.AnahitaNode) error {
	if node.OwnerID <= 0 {
		return errors.New("repost owner ID is invalid")
	}

	if node.OwnerType == "" {
		return errors.New("repost owner type is invalid")
	}

	if node.ParentID <= 0 {
		return errors.New("repost parent ID is invalid")
	}

	if node.ParentType == "" {
		return errors.New("repost parent type is invalid")
	}

	if node.Type != constants.NodeBaseRepostType {
		return errors.New("repost type is invalid")
	}

	if node.Service == "" {
		return errors.New("repost service is invalid")
	}

	if node.CreatedBy <= 0 {
		return errors.New("repost created by is invalid")
	}

	return nil
}
