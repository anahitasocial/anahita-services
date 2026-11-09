package videos

import (
	"encoding/json"
	"net/url"
	"strings"
)

// Media from elsewhere (roadmap rows 120 and 137, docs/plans/audio-and-video.md).
//
// A post from Bluesky or the fediverse that carries a video or a sound
// says, in its meta, where the file is. Nothing of it is copied when the
// post arrives, only its poster, which is the post's picture. The first
// time somebody here presses play, video-service's worker copies it here,
// as one file, and puts it into the post's meta as a video posted here is
// (IntoMeta), so that it plays as one. It is never played from its own
// server, which would tell that server who watched.

// The kinds of file from elsewhere.
const (
	ElsewhereVideo = "video"
	ElsewhereAudio = "audio"
)

// ElsewhereKey is the key of a post's meta that says where its file is.
const ElsewhereKey = "media_elsewhere"

// Elsewhere is a video or a sound on another server.
type Elsewhere struct {
	// ElsewhereVideo or ElsewhereAudio.
	Kind string `json:"kind"`
	// Where the file itself is fetched from: an https address, on the
	// author's own server (a Bluesky author's, their PDS's getBlob).
	Source string `json:"source"`
	// The post's own page there, which is led to when the file cannot be
	// had, and the name of the site it is on ("bsky.app").
	Page string `json:"page"`
	Site string `json:"site"`
	// What the server says the file is, when it says: "video/mp4".
	Type string `json:"type,omitempty"`
}

// Usable says whether it names a file that may be fetched: an https
// address, with no one's name and password in it.
func (e *Elsewhere) Usable() bool {
	if e == nil || (e.Kind != ElsewhereVideo && e.Kind != ElsewhereAudio) {
		return false
	}
	parsed, err := url.Parse(e.Source)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil &&
		len(e.Source) <= 2000 && !strings.ContainsAny(e.Source, " \t\r\n")
}

// Host is the server the file is fetched from.
func (e *Elsewhere) Host() string {
	parsed, err := url.Parse(e.Source)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

// ElsewhereFromMeta is where a post's file is, or nil.
func ElsewhereFromMeta(meta []byte) *Elsewhere {
	raw := field(meta, ElsewhereKey)
	if raw == nil {
		return nil
	}
	elsewhere := &Elsewhere{}
	if err := json.Unmarshal(raw, elsewhere); err != nil || !elsewhere.Usable() {
		return nil
	}
	return elsewhere
}

// WithElsewhere puts where a post's file is into its meta, beside whatever
// else is there; nil leaves the meta as it is.
func WithElsewhere(meta []byte, elsewhere *Elsewhere) []byte {
	if !elsewhere.Usable() {
		return meta
	}
	raw, err := json.Marshal(elsewhere)
	if err != nil {
		return meta
	}
	return withField(meta, ElsewhereKey, raw)
}

// WithoutVideo takes a copied video out of a post's meta, when the copy is
// removed after REMOTE_VIDEO_DAYS; where it is elsewhere stays.
func WithoutVideo(meta []byte) []byte {
	return withField(meta, metaKey, nil)
}

func field(meta []byte, key string) json.RawMessage {
	if len(meta) == 0 {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(meta, &fields); err != nil {
		return nil
	}
	return fields[key]
}

// withField sets one key of a post's meta, or takes it away with nil.
func withField(meta []byte, key string, raw json.RawMessage) []byte {
	fields := map[string]json.RawMessage{}
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &fields)
		if fields == nil {
			fields = map[string]json.RawMessage{}
		}
	}
	if raw == nil {
		delete(fields, key)
	} else {
		fields[key] = raw
	}
	if len(fields) == 0 {
		return nil
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return meta
	}
	return out
}
