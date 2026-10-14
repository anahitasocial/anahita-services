package handlers

import (
	"net/http"
	"strings"
	"testing"

	"anahita-libs/event"
	"anahita-libs/videos"
)

const videoFolder = "assets/v1/video-service/videos/1/abc/"

// anasVideo is a GIF Ana posted, made into a clip, at two sizes; or, with
// heavy, sizes too heavy to send.
func anasVideo(w *inboxWorld, heavy bool) {
	post := w.note()
	post.Type = videos.NodeType
	post.Name = "Beans <growing>"
	post.Body = "Day **three**"
	video := &videos.Video{
		Duration: 2.4, Width: 1920, Height: 1080, Loop: true,
		Poster: videoFolder + "poster.jpg",
		Renditions: []videos.Rendition{
			{Height: 1080, Width: 1920, Key: videoFolder + "1080.mp4", Bytes: 9000000},
			{Height: 720, Width: 1280, Key: videoFolder + "720.mp4", Bytes: 4000000},
		},
	}
	if heavy {
		for i := range video.Renditions {
			video.Renditions[i].Bytes = 90 * 1024 * 1024
		}
	}
	post.Meta, _ = videos.IntoMeta(nil, video)
}

func videoMade(id int64) *event.EventPayload {
	return &event.EventPayload{Name: "node.medium.video.video.added.v1", ObjectID: id, ObjectType: videos.NodeType}
}

// THE RULE THIS GUARDS: a public video post goes out as a note with its
// video attached, at a size other servers take, its poster beside it and its
// title above its words; and its file and poster are given only while the
// post is one that goes out.
func TestAVideoPostGoesOutWithItsVideo(t *testing.T) {
	w := notesWorld()
	anasVideo(w, false)
	files := &fakeFiles{stored: map[string]int{}}
	w.h.Files = files

	if err := w.h.HandleEvent(videoMade(noteID)); err != nil {
		t.Fatal(err)
	}
	if len(w.queue.sent) != 2 {
		t.Fatalf("queued %d", len(w.queue.sent))
	}
	object, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if object["url"] != "https://example.org/videos/500" {
		t.Errorf("url %v", object["url"])
	}
	content, _ := object["content"].(string)
	if !strings.HasPrefix(content, "<p>Beans &lt;growing&gt;</p>") || !strings.Contains(content, "<strong>three</strong>") {
		t.Errorf("content %q", content)
	}
	attachments, _ := object["attachment"].([]any)
	if len(attachments) != 1 {
		t.Fatalf("attachments %v", object["attachment"])
	}
	sent, _ := attachments[0].(map[string]any)
	icon, _ := sent["icon"].(map[string]any)
	if sent["type"] != "Document" || sent["mediaType"] != "video/mp4" || sent["url"] != noteURL+"/video" ||
		sent["width"] != float64(1280) || sent["height"] != float64(720) ||
		icon["url"] != noteURL+"/poster" || icon["mediaType"] != "image/jpeg" {
		t.Errorf("attachment %v", sent)
	}

	// The file asked for is the 720p one, and the poster the poster.
	rec := w.get(w.h.Video, "/", "uuid", noteUUID)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://storage.example/"+videoFolder+"720.mp4?signed" {
		t.Errorf("video: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = w.get(w.h.VideoPoster, "/", "uuid", noteUUID)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://storage.example/"+videoFolder+"poster.jpg?signed" {
		t.Errorf("poster: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	// Closed to followers, it is given to nobody.
	w.note().Access = "followers"
	for name, handler := range map[string]func() int{
		"video":  func() int { return w.get(w.h.Video, "/", "uuid", noteUUID).Code },
		"poster": func() int { return w.get(w.h.VideoPoster, "/", "uuid", noteUUID).Code },
	} {
		if code := handler(); code != http.StatusNotFound {
			t.Errorf("%s of a post for followers: %d", name, code)
		}
	}
}

func TestAVideoTooHeavyToSendGoesAsItsWords(t *testing.T) {
	w := notesWorld()
	anasVideo(w, true)
	w.h.Files = &fakeFiles{stored: map[string]int{}}

	_ = w.h.HandleEvent(videoMade(noteID))
	if len(w.queue.sent) == 0 {
		t.Fatal("nothing was sent")
	}
	object, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if object["attachment"] != nil || object["url"] != "https://example.org/videos/500" {
		t.Errorf("sent %v, at %v", object["attachment"], object["url"])
	}
	if rec := w.get(w.h.Video, "/", "uuid", noteUUID); rec.Code != http.StatusNotFound {
		t.Errorf("a file too heavy was given: %d", rec.Code)
	}
}

func TestAVideoPostWithNoVideoYetDoesNotGoOut(t *testing.T) {
	w := notesWorld()
	w.note().Type = videos.NodeType

	_ = w.h.HandleEvent(videoMade(noteID))
	if len(w.queue.sent) != 0 {
		t.Errorf("queued %d", len(w.queue.sent))
	}
}

func TestTheSizeOfAVideoSent(t *testing.T) {
	video := &videos.Video{Renditions: []videos.Rendition{
		{Height: 1080, Bytes: 50}, {Height: 720, Bytes: 30}, {Height: 360, Bytes: 10},
	}}
	for name, c := range map[string]struct {
		max  int64
		want int
	}{
		"720p fits":            {100, 720},
		"only 360p is light":   {20, 360},
		"none light enough":    {5, 0},
		"no limit, still 720p": {0, 720},
	} {
		got := video.Shared(c.max)
		if (got == nil && c.want != 0) || (got != nil && got.Height != c.want) {
			t.Errorf("%s: %+v", name, got)
		}
	}
	// A clip made of a small GIF is one size, under 720.
	small := &videos.Video{Renditions: []videos.Rendition{{Height: 240, Bytes: 10}}}
	if got := small.Shared(100); got == nil || got.Height != 240 {
		t.Errorf("a small clip: %+v", got)
	}
	// Only sizes taller than 720: the smallest that is light enough.
	tall := &videos.Video{Renditions: []videos.Rendition{{Height: 2160, Bytes: 10}, {Height: 1080, Bytes: 10}}}
	if got := tall.Shared(100); got == nil || got.Height != 1080 {
		t.Errorf("only tall sizes: %+v", got)
	}
}
