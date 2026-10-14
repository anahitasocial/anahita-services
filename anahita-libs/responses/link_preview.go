package responses

import (
	"anahita-libs/data/entities"
	"anahita-libs/federation"
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

// MediaRemoved is what a post from another server says of pictures that
// are no longer stored here: how many there were, what each showed, and
// whether they can still be had from where they came from.
type MediaRemoved struct {
	Count int      `json:"count"`
	Alts  []string `json:"alts"`
	Gone  bool     `json:"gone,omitempty"`
}

// NewMediaRemoved reads it from the post, or answers nil when the post's
// pictures are all there.
func NewMediaRemoved(node *entities.AnahitaNode) *MediaRemoved {
	if node == nil {
		return nil
	}
	removed := federation.MediaRemovedFrom(node.Meta)
	if removed == nil {
		return nil
	}
	return &MediaRemoved{Count: len(removed.Alts), Alts: removed.Alts, Gone: removed.Gone}
}
