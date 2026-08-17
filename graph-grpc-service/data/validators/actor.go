package validators

import (
	"anahita-libs/data/entities"
	"errors"
)

func NewActorValidator() *ActorValidator {
	return &ActorValidator{}
}

type ActorValidator struct{}

func (v *ActorValidator) baseValidation(node *entities.AnahitaNode) error {
	if node.Type == "" {
		return errors.New("type is required")
	}

	if node.Service == "" {
		return errors.New("service is required")
	}

	if node.Name == "" {
		return errors.New("name is required")
	}

	if node.Alias == "" {
		return errors.New("alias is required")
	}

	if node.Body == "" {
		return errors.New("body is required")
	}

	if node.Name != "" && len(node.Name) > 255 {
		return errors.New("name exceeds maximum length of 255 characters")
	}

	if node.Alias != "" && len(node.Alias) > 255 {
		return errors.New("alias exceeds maximum length of 255 characters")
	}

	if node.Access == "" {
		return errors.New("access is required")
	}

	return nil
}

func (v *ActorValidator) ValidateEdit(node *entities.AnahitaNode) error {
	if node.ID <= 0 {
		return errors.New("invalid node ID for note edit")
	}

	if err := v.baseValidation(node); err != nil {
		return err
	}

	if node.ModifiedBy <= 0 {
		return errors.New("invalid modifier ID for note edit")
	}

	return nil
}

func (v *ActorValidator) ValidateAdd(node *entities.AnahitaNode) error {
	if err := v.baseValidation(node); err != nil {
		return err
	}

	if node.CreatedBy <= 0 {
		return errors.New("invalid creator ID for note creation")
	}

	return nil
}
