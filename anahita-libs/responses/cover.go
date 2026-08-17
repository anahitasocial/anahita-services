package responses

import "anahita-libs/data/entities"

func NewCover(cover map[string]entities.AnahitaNodeImage) map[string]Image {
	if cover == nil {
		return nil
	}

	result := make(map[string]Image, len(cover))
	for size, cv := range cover {
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
