// Package videos is what a video post is, to everything that is not the
// service that makes one: the files it has, where they are, and how a post
// says so.
//
// A video post is a node like any other post, with its files described in
// its meta under "video". Nothing but the worker writes that.
package videos

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

const (
	// NodeType is a video post.
	NodeType = "node.medium.video-service.video.v1"
	Service  = "video-service"

	// StoragePrefix is where a worker keeps what it makes; UploadsPrefix,
	// where a browser's upload waits for it. The file service holds each
	// to its place: see file-grpc-service's uploads.go.
	StoragePrefix = "assets/v1/video-service/videos/"
	UploadsPrefix = "assets/v1/video-service/uploads/"

	// Where a post's files are asked for, under the API: addresses that
	// stay the same, each answered with a redirect to a signed one for
	// whoever may see the post.
	FilesPath = "/videos/"

	MimeVideo  = "video/mp4"
	MimePoster = "image/jpeg"

	// The states an upload goes through.
	StatusUploading  = "uploading"
	StatusProcessing = "processing"
	StatusReady      = "ready"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"
)

// Rendition is one size of a video, as it is kept.
type Rendition struct {
	// The height it is named for: 720, 360.
	Height int    `json:"height"`
	Width  int    `json:"width"`
	Key    string `json:"key"`
	Bytes  int64  `json:"bytes"`
}

// Video is what a video post has: its sizes, largest first, and its
// poster, the still shown before it plays.
type Video struct {
	// Seconds.
	Duration   float64     `json:"duration"`
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	Poster     string      `json:"poster"`
	Renditions []Rendition `json:"renditions"`
	// Made of a GIF or an animated PNG: a silent clip played on a loop,
	// with no controls, as the picture it was.
	Loop bool `json:"loop,omitempty"`
}

// Folder is where one video's files are kept: under the profile it was
// posted on, by the id of its upload.
func Folder(ownerID int64, uploadID string) string {
	return fmt.Sprintf("%s%d/%s/", StoragePrefix, ownerID, uploadID)
}

// SourceKey is where an upload waits for a worker.
func SourceKey(personID int64, uploadID string) string {
	return fmt.Sprintf("%s%d/%s/source", UploadsPrefix, personID, uploadID)
}

func RenditionKey(folder string, height int) string {
	return folder + strconv.Itoa(height) + ".mp4"
}

func PosterKey(folder string) string {
	return folder + "poster.jpg"
}

// The key a post's meta keeps its video under.
const metaKey = "video"

// FromMeta reads a post's video out of its meta, or nil when it has none.
func FromMeta(meta []byte) *Video {
	if len(meta) == 0 {
		return nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(meta, &fields); err != nil {
		return nil
	}
	raw, ok := fields[metaKey]
	if !ok {
		return nil
	}

	video := &Video{}
	if err := json.Unmarshal(raw, video); err != nil || len(video.Renditions) == 0 {
		return nil
	}

	return video
}

// IntoMeta puts a video into a post's meta, beside whatever else is there.
func IntoMeta(meta []byte, video *Video) ([]byte, error) {
	fields := map[string]json.RawMessage{}
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &fields)
		if fields == nil {
			fields = map[string]json.RawMessage{}
		}
	}

	raw, err := json.Marshal(video)
	if err != nil {
		return nil, err
	}
	fields[metaKey] = raw

	return json.Marshal(fields)
}

// Keys is every file a video has, for removing them.
func (v *Video) Keys() []string {
	if v == nil {
		return nil
	}

	keys := make([]string, 0, len(v.Renditions)+1)
	for _, rendition := range v.Renditions {
		keys = append(keys, rendition.Key)
	}
	if v.Poster != "" {
		keys = append(keys, v.Poster)
	}
	return keys
}

// SharedHeight is the tallest size of a video sent to another network:
// others make their own smaller ones, and 1080p is heavy to send.
const SharedHeight = 720

// Shared is the size of a video sent to another network, which takes files
// up to maxBytes: the largest no taller than SharedHeight and no heavier
// than that, or failing it the smallest that is light enough. Nil when none
// is, and the video is sent as a link to it instead.
func (v *Video) Shared(maxBytes int64) *Rendition {
	if v == nil {
		return nil
	}
	var lightest *Rendition
	for i := range v.Renditions {
		r := &v.Renditions[i]
		if maxBytes > 0 && r.Bytes > maxBytes {
			continue
		}
		if r.Height <= SharedHeight {
			// Largest first: the first that fits is the one.
			return r
		}
		if lightest == nil || r.Height < lightest.Height {
			lightest = r
		}
	}
	return lightest
}

// Rendition finds a size by its height, or nil.
func (v *Video) Rendition(height int) *Rendition {
	if v == nil {
		return nil
	}
	for i := range v.Renditions {
		if v.Renditions[i].Height == height {
			return &v.Renditions[i]
		}
	}
	return nil
}

var fileName = regexp.MustCompile(`^(\d{2,4})\.mp4$`)

// FileHeight reads the size asked for out of a file's name, "720.mp4".
func FileHeight(name string) (int, bool) {
	found := fileName.FindStringSubmatch(name)
	if found == nil {
		return 0, false
	}
	height, err := strconv.Atoi(found[1])
	return height, err == nil
}

// FilePath is the address a size is asked for at, and PosterPath its
// poster's.
func FilePath(nodeID int64, height int) string {
	return fmt.Sprintf("%s%d/files/%d.mp4", FilesPath, nodeID, height)
}

func PosterPath(nodeID int64) string {
	return fmt.Sprintf("%s%d/files/poster.jpg", FilesPath, nodeID)
}
