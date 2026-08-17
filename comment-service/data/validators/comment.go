package validators

import (
	"anahita-libs/data/entities"
	"comment/constants"
	"errors"
	"fmt"
)

func NewCommentValidator() *CommentValidator {
	return &CommentValidator{}
}

type CommentValidator struct{}

func (v *CommentValidator) baseValidation(node *entities.AnahitaNode) error {
	if node.Type == "" {
		return errors.New("type is required")
	}

	if node.Service == "" {
		return errors.New("service is required")
	}

	if node.Body == "" {
		return errors.New("body is required")
	}

	if len(node.Body) > constants.MaxCommentBodyLength {
		return fmt.Errorf("body exceeds maximum length of %d characters", constants.MaxCommentBodyLength)
	}

	if node.ParentID <= 0 {
		return errors.New("parent ID is required")
	}

	if node.ParentType == "" {
		return errors.New("parent type is required")
	}

	return nil
}

func (v *CommentValidator) ValidateEdit(node *entities.AnahitaNode) error {
	if node.ID <= 0 {
		return nil
	}

	if err := v.baseValidation(node); err != nil {
		return err
	}

	if node.ModifiedBy <= 0 {
		return errors.New("invalid modifier ID for comment edit")
	}

	return nil
}

func (v *CommentValidator) ValidateAdd(node *entities.AnahitaNode) error {
	if err := v.baseValidation(node); err != nil {
		return err
	}

	if node.CreatedBy <= 0 {
		return errors.New("invalid creator ID for comment add")
	}

	return nil
}
