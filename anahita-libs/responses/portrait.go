package responses

import "anahita-libs/data/entities"

func NewPortrait(portrait map[string]entities.AnahitaNodeImage) map[string]Image {
	if portrait == nil {
		return nil
	}

	result := make(map[string]Image, len(portrait))
	for size, cv := range portrait {
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
