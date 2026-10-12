package responses

import (
	"anahita-libs/data/entities"
	"anahita-libs/videos"
)

// VideoFile is one size of a video.
type VideoFile struct {
	Height int `json:"height"`
	Width  int `json:"width"`
	// Where it is read from: a path under the API, the same for as long as
	// the post is there, answered for whoever may see the post.
	URL  string `json:"url"`
	Type string `json:"type"`
}

// Video is what a video post plays: its sizes, largest first, and the
// still shown before it does.
type Video struct {
	Duration float64      `json:"duration"`
	Width    int          `json:"width"`
	Height   int          `json:"height"`
	Poster   string       `json:"poster,omitempty"`
	Files    []*VideoFile `json:"files"`
}

// NewVideo reads a post's video out of its meta, or nil when it is not a
// video post.
func NewVideo(node *entities.AnahitaNode) *Video {
	if node == nil || node.Type != videos.NodeType {
		return nil
	}

	video := videos.FromMeta(node.Meta)
	if video == nil {
		return nil
	}

	out := &Video{
		Duration: video.Duration,
		Width:    video.Width,
		Height:   video.Height,
		Files:    make([]*VideoFile, 0, len(video.Renditions)),
	}
	if video.Poster != "" {
		out.Poster = videos.PosterPath(node.ID)
	}
	for _, rendition := range video.Renditions {
		out.Files = append(out.Files, &VideoFile{
			Height: rendition.Height,
			Width:  rendition.Width,
			URL:    videos.FilePath(node.ID, rendition.Height),
			Type:   videos.MimeVideo,
		})
	}

	return out
}
