package validators

import (
	"anahita-libs/data/entities"
	"errors"
)

func NewMediumValidator() *MediumValidator {
	return &MediumValidator{}
}

type MediumValidator struct{}

func (v *MediumValidator) baseValidation(node *entities.AnahitaNode) error {
	if node.Type == "" {
		return errors.New("type is required")
	}

	if node.Service == "" {
		return errors.New("service is required")
	}

	// Something to show: a title, words, or a file. A post of pictures or a
	// video needs neither title nor words, as a reply with pictures never
	// had: refused here, it failed with a 500 after its pictures were
	// stored.
	// A file is known by its name, or, for a video the worker posts before
	// its files are named, by its type.
	if node.Name == "" && node.Body == "" && node.Filename == "" && node.Mimetype == "" {
		return errors.New("either name, body or a file is required")
	}

	if node.Name != "" && len(node.Name) > 255 {
		return errors.New("name exceeds maximum length of 255 characters")
	}

	if node.Alias != "" && len(node.Alias) > 255 {
		return errors.New("alias exceeds maximum length of 255 characters")
	}

	if node.OwnerID <= 0 {
		return errors.New("owner ID is required")
	}

	if node.OwnerType == "" {
		return errors.New("owner type is required")
	}

	return nil
}

func (v *MediumValidator) ValidateEdit(node *entities.AnahitaNode) error {
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

func (v *MediumValidator) ValidateAdd(node *entities.AnahitaNode) error {
	if err := v.baseValidation(node); err != nil {
		return err
	}

	if node.CreatedBy <= 0 {
		return errors.New("invalid creator ID for note creation")
	}

	return nil
}
