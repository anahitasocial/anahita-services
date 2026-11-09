package responses

import (
	"anahita-libs/data/entities"
	"anahita-libs/videos"
)

// VideoFile is one size of a video.
type VideoFile struct {
	// In pixels.
	Height int `json:"height"`
	// In pixels.
	Width int `json:"width"`
	// Where it is read from: a path under the API, the same for as long as
	// the post is there, answered for whoever may see the post.
	URL string `json:"url"`
	// The kind of file: `video/mp4`.
	Type string `json:"type"`
}

// Video is what a video post plays: its sizes, largest first, and the
// still shown before it does.
type Video struct {
	// How long it plays, in seconds.
	Duration float64 `json:"duration"`
	// In pixels: its shape, which a player can be laid out by before it loads.
	Width int `json:"width"`
	// In pixels.
	Height int `json:"height"`
	// The picture shown before it plays: a path under the API.
	Poster string `json:"poster,omitempty"`
	// Each size it can be played in, the largest first.
	Files []*VideoFile `json:"files"`
	// A GIF or an animated PNG: played silently, on a loop.
	Loop bool `json:"loop,omitempty"`
}

// NewVideo reads a post's video out of its meta, or nil when it is not a
// video post.
func NewVideo(node *entities.AnahitaNode) *Video {
	// A video post, or a post from elsewhere whose video was copied here
	// when it was first played (roadmap row 120).
	if node == nil || (node.Type != videos.NodeType && videos.ElsewhereFromMeta(node.Meta) == nil) {
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
		Loop:     video.Loop,
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

// Elsewhere is a video or a sound a post from another network carries,
// not kept here until somebody plays it (roadmap rows 120 and 137): what
// it is, and the page it is on there. Played by asking
// `POST /videos/elsewhere/:id`.
type Elsewhere struct {
	// `video` or `audio`.
	Kind string `json:"kind"`
	// The post's page on its own site, led to when it cannot be had.
	Page string `json:"page"`
	// The name of that site: `bsky.app`.
	Site string `json:"site"`
}

// NewElsewhere is what a post says of its file from elsewhere, or nil.
func NewElsewhere(node *entities.AnahitaNode) *Elsewhere {
	if node == nil {
		return nil
	}
	elsewhere := videos.ElsewhereFromMeta(node.Meta)
	if elsewhere == nil {
		return nil
	}
	return &Elsewhere{Kind: elsewhere.Kind, Page: elsewhere.Page, Site: elsewhere.Site}
}

// Audio is what an audio post plays (roadmap row 137): its one file, how
// long it plays, and its cover when it has one.
type Audio struct {
	// How long it plays, in seconds.
	Duration float64 `json:"duration"`
	// Where it is read from: a path under the API, answered for whoever
	// may see the post.
	URL string `json:"url"`
	// The kind of file: `audio/mp4`.
	Type string `json:"type"`
	// Its cover, a path under the API, when the file carried one.
	Cover string `json:"cover,omitempty"`
}

// NewAudio reads an audio post's sound out of its meta, or nil when it is
// not an audio post.
func NewAudio(node *entities.AnahitaNode) *Audio {
	if node == nil || node.Type != videos.AudioNodeType {
		return nil
	}
	audio := videos.AudioFromMeta(node.Meta)
	if audio == nil {
		return nil
	}
	out := &Audio{
		Duration: audio.Duration,
		URL:      videos.NamedPath(node.ID, videos.AudioFile),
		Type:     videos.MimeAudio,
	}
	if audio.Cover != "" {
		out.Cover = videos.NamedPath(node.ID, videos.CoverFile)
	}
	return out
}
