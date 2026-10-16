package responses

// ImageSize is how big an image is.
type ImageSize struct {
	// In pixels.
	Width int `json:"width"`
	// In pixels.
	Height int `json:"height"`
}

// Image is one size of a picture.
type Image struct {
	// Where it is read from.
	Url string `json:"url"`
	// How big it is.
	Size ImageSize `json:"size"`
}
