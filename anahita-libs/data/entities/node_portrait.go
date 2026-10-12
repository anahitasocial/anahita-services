package entities

import (
	"fmt"
	"log"
	"strings"
)

const (
	PortraitSizeSquare   = "square"
	PortraitSizeSmall    = "small"
	PortraitSizeMedium   = "medium"
	PortraitSizeLarge    = "large"
	PortraitSizeXLarge   = "xlarge"
	PortraitSizeXXLarge  = "xxlarge"
	PortraitSizeOriginal = "original"
)

type AnahitaNodePortraitInterface interface {
	GetPortraitFile(size string) (string, error)
	GetPortraitUrl(size string) (string, error)
	GetPortraitSizes() []string
	IsValidPortraitSize(size string) bool
	GetPortraits() (map[string]AnahitaNodeImage, error)
	GetPortrait(size string) (AnahitaNodeImage, error)
	SetPortrait(portrait AnahitaNodeImage, size string) error
}

// get Image File
func (a *AnahitaNode) GetPortraitFile(size string) (filePath string, err error) {
	if a.Portrait == nil {
		log.Println("Portrait is nil")
		return "", fmt.Errorf("portrait is nil")
	}

	if a.GetFilename() == "" {
		log.Println("Filename is empty")
		return "", fmt.Errorf("filename is empty")
	}

	name := strings.Split(a.GetFilename(), ".")

	if len(name) != 2 {
		log.Println("Invalid filename")
		return "", fmt.Errorf("invalid filename")
	}

	filePath = fmt.Sprintf("%s_%s.%s", name[0], size, name[1])

	return filePath, nil
}

// get Image URL
func (a *AnahitaNode) GetPortraitUrl(size string) (url string, err error) {
	file, err := a.GetPortraitFile(size)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%s/%s/%d/%s", AssetsPrefix, a.Service, a.GetEntityName(), a.OwnerID, file), nil
}

// get Image Sizes
func (a *AnahitaNode) GetPortraitSizes() []string {
	return []string{
		PortraitSizeSquare,
		PortraitSizeSmall,
		PortraitSizeMedium,
		PortraitSizeLarge,
		PortraitSizeXLarge,
		PortraitSizeXXLarge,
		PortraitSizeOriginal,
	}
}

// IsValidPortraitSize
func (a *AnahitaNode) IsValidPortraitSize(size string) bool {
	sizes := a.GetPortraitSizes()
	for _, s := range sizes {
		if s == size {
			return true
		}
	}

	return false
}

func (a *AnahitaNode) GetPortraits() (map[string]AnahitaNodeImage, error) {
	sizes := a.GetPortraitSizes()
	a.Portrait = make(map[string]AnahitaNodeImage)

	for _, size := range sizes {
		var width, height int = 0, 0
		switch size {

		case PortraitSizeXXLarge:
			width = 80 * 24 // 1920
			height = 0
		case PortraitSizeXLarge:
			width = 80 * 20 // 1600
			height = 0
		case PortraitSizeLarge:
			width = 80 * 16 // 1280
			height = 0
		case PortraitSizeMedium:
			width = 80 * 12 // 960
			height = 0
		case PortraitSizeSmall:
			width = 80 * 8 // 640
			height = 0
		case PortraitSizeSquare:
			width = 80 * 8  // 640
			height = 80 * 8 // 640
		default:
			width = 80 * 4  // 320
			height = 80 * 4 // 320
		}

		url, err := a.GetPortraitUrl(size)
		if err != nil {
			return nil, err
		}

		a.Portrait[size] = AnahitaNodeImage{
			Url: url,
			Size: AnahitaNodeImageSize{
				Width:  width,
				Height: height,
			},
		}
	}

	return a.Portrait, nil
}

// Get Portrait
func (a *AnahitaNode) GetPortrait(size string) (AnahitaNodeImage, error) {
	if !a.IsValidPortraitSize(size) {
		return AnahitaNodeImage{}, fmt.Errorf("invalid portrait size")
	}

	return a.Portrait[size], nil
}

// Set Cover
func (a *AnahitaNode) SetPortrait(portrait AnahitaNodeImage, size string) error {
	if !a.IsValidPortraitSize(size) {
		return fmt.Errorf("invalid portrait size")
	}

	a.Portrait[size] = portrait
	return nil
}
