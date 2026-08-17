package validators

import (
	"anahita-libs/data/entities"
	"errors"
)

func NewLocationValidator() *LocationValidator {
	return &LocationValidator{}
}

type LocationValidator struct{}

func (v *LocationValidator) baseValidation(node *entities.AnahitaNode) error {
	if node.Type == "" {
		return errors.New("type is required")
	}

	if node.Service == "" {
		return errors.New("service is required")
	}

	if node.Name == "" && node.Body == "" {
		return errors.New("either name or body are required")
	}

	if node.Name != "" && len(node.Name) > 255 {
		return errors.New("name exceeds maximum length of 255 characters")
	}

	if node.Alias != "" && len(node.Alias) > 255 {
		return errors.New("alias exceeds maximum length of 255 characters")
	}

	return nil
}

func (v *LocationValidator) ValidateEdit(node *entities.AnahitaNode) error {
	if node.ID <= 0 {
		return errors.New("invalid node ID for note edit")
	}

	if err := v.baseValidation(node); err != nil {
		return err
	}

	if node.ModifiedBy <= 0 {
		return errors.New("invalid modifier ID for note edit")
	}

	if node.GeoLatitude < -90 || node.GeoLatitude > 90 {
		return errors.New("latitude must be between -90 and 90 degrees")
	}

	if node.GeoLongitude < -180 || node.GeoLongitude > 180 {
		return errors.New("longitude must be between -180 and 180 degrees")
	}

	return nil
}

func (v *LocationValidator) ValidateAdd(node *entities.AnahitaNode) error {
	if err := v.baseValidation(node); err != nil {
		return err
	}

	if node.CreatedBy <= 0 {
		return errors.New("invalid creator ID for note creation")
	}

	if node.GeoAddress == "" && node.GeoCity == "" && node.GeoStateProvince == "" && node.GeoCountry == "" && node.GeoPostalcode == "" {
		return errors.New("at least one geographic field (address, city, state/province, country, or postal code) is required")
	}

	return nil
}
