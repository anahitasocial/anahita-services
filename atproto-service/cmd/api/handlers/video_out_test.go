package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/videos"
)

// fakeTransfers gives the address of a little server that answers with
// what is kept under each key.
type fakeTransfers struct {
	services.FileTransfersInterface
	server *httptest.Server
	kept   map[string][]byte
	asked  []string
}

func newTransfers(t *testing.T) *fakeTransfers {
	f := &fakeTransfers{kept: map[string][]byte{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, ok := f.kept[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(content)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeTransfers) FetchURL(key string) (string, error) {
	f.asked = append(f.asked, key)
	return f.server.URL + "/" + key, nil
}

const clipFolder = "assets/v1/video-service/videos/1/abc/"

func aClipPost(id int64, duration float64) *entities.AnahitaNode {
	meta, _ := videos.IntoMeta(nil, &videos.Video{
		Duration: duration, Width: 1920, Height: 1080, Loop: true, Poster: clipFolder + "poster.jpg",
		Renditions: []videos.Rendition{
			{Height: 1080, Width: 1920, Key: clipFolder + "1080.mp4", Bytes: 9},
			{Height: 720, Width: 1280, Key: clipFolder + "720.mp4", Bytes: 5},
		},
	})
	return &entities.AnahitaNode{
		ID: id, Type: videos.NodeType, Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Name: "Beans growing", Body: "Day three", Meta: meta,
	}
}

func writtenPost(t *testing.T, w *writing) string {
	t.Helper()
	if len(w.sent.rows) == 0 {
		t.Fatal("nothing was written")
	}
	out, _ := json.Marshal(w.writer.records["app.bsky.feed.post/"+w.sent.rows[0].RKey])
	return string(out)
}

// THE RULE THIS GUARDS: a video post goes to Bluesky with its video, the
// 720p size read from storage and uploaded, its title above its words.
func TestAVideoPostGoesWithItsVideo(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()
	transfers := newTransfers(t)
	transfers.kept[clipFolder+"720.mp4"] = []byte("clip!")
	w.h.Transfers = transfers

	w.nodes.kept = append(w.nodes.kept, aClipPost(91, 2.4))
	w.heard("node.medium.video-service.video.added.v1", event.EventPayload{ObjectType: videos.NodeType, ObjectID: 91})
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}

	out := writtenPost(t, w)
	for _, want := range []string{
		`"text":"Beans growing\n\nDay three"`,
		`"embed":{"$type":"app.bsky.embed.video","video":{"$type":"blob","ref":{"$link":"blob1"},"mimeType":"video/mp4","size":5}`,
		`"aspectRatio":{"width":1280,"height":720}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the video post lacks %s:\n%s", want, out)
		}
	}
	if len(transfers.asked) != 1 || transfers.asked[0] != clipFolder+"720.mp4" {
		t.Errorf("read %v", transfers.asked)
	}
}

// A video longer than Bluesky takes goes as a card that leads to it, with
// its poster; nothing of the video is read.
func TestAVideoTooLongForBlueskyGoesAsACard(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()
	transfers := newTransfers(t)
	w.h.Transfers = transfers
	w.files.content[clipFolder+"poster.jpg"] = aPicture(t)

	w.nodes.kept = append(w.nodes.kept, aClipPost(92, 600))
	w.heard("node.medium.video-service.video.added.v1", event.EventPayload{ObjectType: videos.NodeType, ObjectID: 92})
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}

	out := writtenPost(t, w)
	for _, want := range []string{
		`"$type":"app.bsky.embed.external"`, `"uri":"https://anahita.example/videos/92"`, `"title":"Beans growing"`,
		`"thumb":{"$type":"blob","ref":{"$link":"blob1"}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the card lacks %s:\n%s", want, out)
		}
	}
	if len(transfers.asked) != 0 {
		t.Errorf("the video was read: %v", transfers.asked)
	}
}

func TestAVideoPostWithNoVideoYetIsNotWritten(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()
	post := aClipPost(93, 2)
	post.Meta = nil
	w.nodes.kept = append(w.nodes.kept, post)

	w.heard("node.medium.video-service.video.added.v1", event.EventPayload{ObjectType: videos.NodeType, ObjectID: 93})
	if len(w.queue.waiting) != 0 {
		t.Errorf("waiting %d", len(w.queue.waiting))
	}
}
