package responses

import "anahita-libs/data/entities"

func NewAvatar(avatar map[string]entities.AnahitaNodeImage) map[string]Image {
	if avatar == nil {
		return nil
	}

	result := make(map[string]Image, len(avatar))
	for size, cv := range avatar {
		result[size] = Image{
			Url: cv.Url,
			Size: ImageSize{
				Width:  cv.Size.Width,
				Height: cv.Size.Height,
			},
		}
	}
	return result
}
