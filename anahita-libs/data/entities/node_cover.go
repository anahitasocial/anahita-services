package entities

import (
	"fmt"
	"strings"
)

const (
	CoverSizeMedium = "medium"
	CoverSizeLarge  = "large"
)

type AnahitaNodeCoverInterface interface {
	GetCoverFile(size string) string
	GetCoverURL(size string) string
	GetCoverSizes() []string
	IsValidCoverSize(size string) bool
	GetCovers() (map[string]AnahitaNodeImage, error)
	GetCover(size string) (AnahitaNodeImage, error)
	SetCover(cover AnahitaNodeImage, size string) error
}

// GetCoverFile returns the cover file name
func (a *AnahitaNode) GetCoverFile(size string) string {
	if a.CoverFilename == "" {
		return ""
	}

	name := strings.Split(a.CoverFilename, ".")
	return fmt.Sprintf("%s_%s.%s", name[0], size, name[1])
}

// Get Cover Url
func (a *AnahitaNode) GetCoverURL(size string) string {
	file := a.GetCoverFile(size)

	if file == "" {
		return ""
	}

	id := a.ID
	if a.OwnerID != 0 {
		id = a.OwnerID
	}

	return fmt.Sprintf("assets/v2/cover-service/%s/%d/%s", a.GetEntityName(), id, file)
}

// Get Cover Sizes
func (a *AnahitaNode) GetCoverSizes() []string {
	return []string{
		CoverSizeMedium,
		CoverSizeLarge,
	}
}

// IsValidCoverSize
func (a *AnahitaNode) IsValidCoverSize(size string) bool {
	sizes := a.GetCoverSizes()

	for _, s := range sizes {
		if s == size {
			return true
		}
	}

	return false
}

// Get Covers
func (a *AnahitaNode) GetCovers() (map[string]AnahitaNodeImage, error) {
	sizes := []string{"medium", "large"}
	a.Cover = make(map[string]AnahitaNodeImage)

	for _, size := range sizes {
		var width, height int
		switch size {
		case CoverSizeMedium:
			width = 640
			height = 0
		case CoverSizeLarge:
			width = 1600
			height = 0
		default:
			width = 1600
			height = 0
		}

		a.Cover[size] = AnahitaNodeImage{
			Url: a.GetCoverURL(size),
			Size: AnahitaNodeImageSize{
				Width:  width,
				Height: height,
			},
		}
	}

	return a.Cover, nil
}

// Get Cover
func (a *AnahitaNode) GetCover(size string) (AnahitaNodeImage, error) {
	if !a.IsValidCoverSize(size) {
		return AnahitaNodeImage{}, fmt.Errorf("invalid cover size")
	}

	return a.Cover[size], nil
}

// Set Cover
func (a *AnahitaNode) SetCover(cover AnahitaNodeImage, size string) error {
	if !a.IsValidCoverSize(size) {
		return fmt.Errorf("invalid cover size")
	}

	a.Cover[size] = cover
	return nil
}
