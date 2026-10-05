package entities

import (
	"fmt"
	"slices"
	"strings"
)

const (
	AvatarSizeSmall  = "small"
	AvatarSizeMedium = "medium"
	AvatarSizeLarge  = "large"
	AvatarSizeSquare = "square"
)

type AnahitaNodeAvatarInterface interface {
	GetAvatarFile(size string) string
	GetAvatarUrl(size string) string
	GetAvatarSizes() []string
	IsValidAvatarSize(size string) bool
	GetAvatars() (map[string]AnahitaNodeImage, error)
	GetAvatar(size string) (AnahitaNodeImage, error)
	SetAvatar(avatar AnahitaNodeImage, size string) error
	GetWidth(size string) int
	GetHeight(size string) int
}

// Get Avatar File Path
func (a *AnahitaNode) GetAvatarFile(size string) string {
	if a.GetAvatarFilename() == "" {
		return ""
	}

	name := strings.Split(a.GetAvatarFilename(), ".")
	return fmt.Sprintf("%s_%s.%s", name[0], size, name[1])
}

// GetAvatarFile
func (a *AnahitaNode) GetAvatarUrl(size string) string {
	file := a.GetAvatarFile(size)

	if file == "" {
		return ""
	}

	return fmt.Sprintf("assets/v2/avatar-service/%s/%d/%s", a.GetEntityName(), a.ID, file)
}

// Get Avatar Sizes
func (a *AnahitaNode) GetAvatarSizes() []string {
	return []string{
		AvatarSizeSmall,
		AvatarSizeMedium,
		AvatarSizeLarge,
		AvatarSizeSquare,
	}
}

// Is Valid Size
func (a *AnahitaNode) IsValidAvatarSize(size string) bool {
	sizes := a.GetAvatarSizes()
	return slices.Contains(sizes, size)
}

// get Avatars
func (a *AnahitaNode) GetAvatars() (map[string]AnahitaNodeImage, error) {
	sizes := a.GetAvatarSizes()
	a.Avatar = make(map[string]AnahitaNodeImage)

	for _, size := range sizes {
		var width, height int
		switch size {
		case AvatarSizeLarge:
			width = 80 * 6 // 480
			height = 0
		case AvatarSizeMedium:
			width = 80 * 4 // 160
			height = 0
		case AvatarSizeSmall:
			width = 80 * 2 // 160
			height = 0
		case AvatarSizeSquare:
			width = 80 * 2  // 160
			height = 80 * 2 // 160
		default:
			width = 80 * 1
			height = 80 * 1
		}

		a.Avatar[size] = AnahitaNodeImage{
			Url: a.GetAvatarUrl(size),
			Size: AnahitaNodeImageSize{
				Width:  width,
				Height: height,
			},
		}
	}

	return a.Avatar, nil
}

// Get Avatar
func (a *AnahitaNode) GetAvatar(size string) (AnahitaNodeImage, error) {
	if !a.IsValidAvatarSize(size) {
		return AnahitaNodeImage{}, fmt.Errorf("invalid avatar size")
	}

	return a.Avatar[size], nil
}

// Set Avatar
func (a *AnahitaNode) SetAvatar(avatar AnahitaNodeImage, size string) error {
	if !a.IsValidAvatarSize(size) {
		return fmt.Errorf("invalid avatar size")
	}

	a.Avatar[size] = avatar

	return nil
}

// GetAvatarWidth
func (a *AnahitaNode) GetWidth(size string) int {
	if !a.IsValidAvatarSize(size) {
		return 0
	}

	if a.Avatar == nil {
		return 0
	}

	avatar, exists := a.Avatar[size]
	if !exists {
		return 0
	}

	return avatar.Size.Width
}

// GetAvatarHeight
func (a *AnahitaNode) GetHeight(size string) int {
	if !a.IsValidAvatarSize(size) {
		return 0
	}

	if a.Avatar == nil {
		return 0
	}

	avatar, exists := a.Avatar[size]
	if !exists {
		return 0
	}

	return avatar.Size.Height
}
