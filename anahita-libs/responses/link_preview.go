package responses

import (
	"anahita-libs/data/entities"
	"anahita-libs/linkpreviews"
)

// LinkPreview is the card a note carries for a link in it: what the page
// was called and was about when the note was written.
type LinkPreview struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	// Where its picture is read from, on this installation: a path under
	// the API. Never the other site's own address for it.
	Image string `json:"image,omitempty"`
}

// NewLinkPreview reads a note's card out of its meta, or nil when it has
// none.
func NewLinkPreview(node *entities.AnahitaNode) *LinkPreview {
	if node == nil {
		return nil
	}

	snapshot := linkpreviews.FromMeta(node.Meta)
	if snapshot == nil {
		return nil
	}

	preview := &LinkPreview{
		URL:         snapshot.URL,
		Title:       snapshot.Title,
		Description: snapshot.Description,
		SiteName:    snapshot.SiteName,
	}
	if snapshot.ImageKey != "" {
		preview.Image = linkpreviews.ImagePath + snapshot.ImageKey
	}

	return preview
}
