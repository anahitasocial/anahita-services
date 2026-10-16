package responses

import "anahita-libs/data/entities"

// PhotoFile is one image of a photo post.
type PhotoFile struct {
	// What an edit names the image by. The stored name, which is already in
	// every address below.
	ID string `json:"id"`
	// Where it comes among the post's pictures, from 0.
	Position int `json:"position"`
	// What the image shows, for people who cannot see it. Always sent, empty
	// when there is none, so a client can tell "none" from "not told".
	AltText string `json:"alt_text"`
	// Of the image as uploaded. Left out for an image stored before these
	// were recorded.
	Width int `json:"width,omitempty"`
	// Of the image as uploaded, in pixels.
	Height int `json:"height,omitempty"`
	// Each stored size, as portrait_urls is for the first image.
	Urls map[string]Image `json:"urls,omitempty"`
}

// NewPhotoFiles maps the images of a photo post. nil when there are none, so
// the field is left out of anything that is not a photo.
func NewPhotoFiles(files []*entities.PhotoFile) []*PhotoFile {
	if len(files) == 0 {
		return nil
	}

	out := make([]*PhotoFile, 0, len(files))
	for _, file := range files {
		out = append(out, &PhotoFile{
			ID:       file.Filename,
			Position: file.Position,
			AltText:  file.AltText,
			Width:    file.Width,
			Height:   file.Height,
			Urls:     NewPortrait(file.Portrait),
		})
	}
	return out
}
