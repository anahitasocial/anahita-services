package videos

import "encoding/json"

// Audio posts (roadmap row 137, docs/plans/audio-and-video.md): a sound as
// a post of its own kind, made and kept by video-service, which has the
// worker and ffmpeg. Uploaded as a video is, and made by the worker into
// one file, AAC in an M4A, with the picture the file carried (an album's
// cover) as its cover when it had one.

const (
	// AudioNodeType is an audio post.
	AudioNodeType = "node.medium.video-service.audio.v1"

	MimeAudio = "audio/mp4"

	// The names of an audio post's files, under its folder.
	AudioFile = "audio.m4a"
	CoverFile = "cover.jpg"

	// The kinds of upload: what the worker makes of it.
	KindVideo = "video"
	KindAudio = "audio"
)

// IsOurs says whether a post is one of video-service's: a video post or an
// audio post, whose routes, files and settings are the same.
func IsOurs(nodeType string) bool {
	return nodeType == NodeType || nodeType == AudioNodeType
}

// Audio is what an audio post has: its one file, how long it plays, and
// its cover, when it has one.
type Audio struct {
	// Seconds.
	Duration float64 `json:"duration"`
	Key      string  `json:"key"`
	Bytes    int64   `json:"bytes"`
	Cover    string  `json:"cover,omitempty"`
}

const audioKey = "audio"

// AudioFromMeta reads an audio post's sound out of its meta, or nil.
func AudioFromMeta(meta []byte) *Audio {
	raw := field(meta, audioKey)
	if raw == nil {
		return nil
	}
	audio := &Audio{}
	if err := json.Unmarshal(raw, audio); err != nil || audio.Key == "" {
		return nil
	}
	return audio
}

// AudioIntoMeta puts a sound into a post's meta, beside whatever else is
// there.
func AudioIntoMeta(meta []byte, audio *Audio) ([]byte, error) {
	raw, err := json.Marshal(audio)
	if err != nil {
		return nil, err
	}
	return withField(meta, audioKey, raw), nil
}

// Keys is every file an audio post has, for removing them.
func (a *Audio) Keys() []string {
	if a == nil {
		return nil
	}
	keys := []string{a.Key}
	if a.Cover != "" {
		keys = append(keys, a.Cover)
	}
	return keys
}

// AudioKey and CoverKey are where an audio post's files are kept.
func AudioKey(folder string) string { return folder + AudioFile }
func CoverKey(folder string) string { return folder + CoverFile }

// FilesOf is every file a post of video-service's keeps, whichever kind it
// is: for the sweep that keeps what is named, and for removing a post.
func FilesOf(meta []byte) []string {
	return append(FromMeta(meta).Keys(), AudioFromMeta(meta).Keys()...)
}
