package responses

type ImageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Image struct {
	Url  string    `json:"url"`
	Size ImageSize `json:"size"`
}
