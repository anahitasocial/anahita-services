package worker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/videos"

	"video/data/repositories"
	"video/encode"
)

type fakeElsewhere struct {
	copies   map[int64]*repositories.Copy
	finished map[int64][2]string
}

func (f *fakeElsewhere) Ask(id int64, kind, source string) (*repositories.Copy, error) {
	return nil, nil
}
func (f *fakeElsewhere) Read(id int64) (*repositories.Copy, error) { return f.copies[id], nil }
func (f *fakeElsewhere) Claim() (*repositories.Copy, error) {
	for _, c := range f.copies {
		if c.Status == repositories.ElsewhereWaiting {
			c.Status = repositories.ElsewhereWorking
			return c, nil
		}
	}
	return nil, nil
}
func (f *fakeElsewhere) Finish(id int64, status, reason string) error {
	f.finished[id] = [2]string{status, reason}
	if c := f.copies[id]; c != nil {
		c.Status = status
	}
	return nil
}
func (f *fakeElsewhere) Unplayed(before time.Time, limit int) ([]*repositories.Copy, error) {
	var found []*repositories.Copy
	for _, c := range f.copies {
		if c.Status == repositories.ElsewhereReady && c.PlayedAt.Before(before) {
			found = append(found, c)
		}
	}
	return found, nil
}

type fakeNodes struct {
	posts map[int64]*entities.AnahitaNode
}

func (f *fakeNodes) Read(id int64) (*entities.AnahitaNode, error) {
	if post, ok := f.posts[id]; ok {
		copied := *post
		return &copied, nil
	}
	return nil, nil
}
func (f *fakeNodes) UpsertRemote(node *entities.AnahitaNode) (*entities.AnahitaNode, bool, error) {
	for id, post := range f.posts {
		if post.URI == node.URI {
			post.Meta = node.Meta
			return f.posts[id], false, nil
		}
	}
	return nil, false, nil
}

// elsewhereFixture is a post from Bluesky whose video is on its author's
// server, a TLS server here, and somebody having pressed play.
func elsewhereFixture(t *testing.T, path string, info *encode.Info) (*fixture, *fakeElsewhere, *fakeNodes) {
	t.Helper()
	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v.mp4" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("their video"))
	}))
	t.Cleanup(other.Close)

	post := &entities.AnahitaNode{ID: 77, OwnerID: 12, URI: "at://did:plc:a/app.bsky.feed.post/1", Type: "node.medium.photo-service.photo.v1"}
	post.Meta = videos.WithElsewhere([]byte(`{"link_preview":{"url":"https://a.example"}}`), &videos.Elsewhere{
		Kind: videos.ElsewhereVideo, Source: other.URL + path, Page: "https://bsky.app/p/1", Site: "bsky.app",
	})
	nodes := &fakeNodes{posts: map[int64]*entities.AnahitaNode{77: post}}
	copies := &fakeElsewhere{
		copies:   map[int64]*repositories.Copy{77: {NodeID: 77, Kind: videos.ElsewhereVideo, Status: repositories.ElsewhereWaiting}},
		finished: map[int64][2]string{},
	}

	f := newFixture(t)
	// No upload waiting: the worker turns to what was asked for from elsewhere.
	f.uploads.claimed = true
	f.encoder.info = info
	f.worker.Elsewhere = copies
	f.worker.Nodes = nodes
	f.worker.RemoteClient = other.Client()
	f.worker.RemoteMaxBytes = 1 << 20
	f.worker.RemoteMaxSeconds = 3600
	return f, copies, nodes
}

// THE RULE THIS GUARDS (roadmap row 120): a video from elsewhere, asked
// for, is fetched from its own server and kept here as one file, copied as
// it is when it already plays everywhere, into the post's meta beside what
// was there; nothing of it is played from the other server.
func TestAVideoFromElsewhereIsCopiedOnceAsItIs(t *testing.T) {
	f, copies, nodes := elsewhereFixture(t, "/v.mp4", &encode.Info{
		Width: 1280, Height: 720, Duration: 30, HasAudio: true, VideoCodec: "h264", AudioCodec: "aac", PixFmt: "yuv420p",
	})

	if more, err := f.worker.Once(); err != nil || !more {
		t.Fatalf("nothing taken: %v", err)
	}
	if got := copies.finished[77]; got != [2]string{repositories.ElsewhereReady, ""} {
		t.Fatalf("finished: %v", got)
	}
	if f.encoder.remuxed != 1 {
		t.Errorf("it was encoded again: %d copied as it is", f.encoder.remuxed)
	}
	video := videos.FromMeta(nodes.posts[77].Meta)
	if video == nil || len(video.Renditions) != 1 || video.Renditions[0].Height != 720 || video.Duration != 30 {
		t.Fatalf("in the post: %+v", video)
	}
	if !strings.HasPrefix(video.Renditions[0].Key, videos.StoragePrefix+"12/elsewhere-77/") {
		t.Errorf("stored at %s", video.Renditions[0].Key)
	}
	if !strings.Contains(string(nodes.posts[77].Meta), "link_preview") || videos.ElsewhereFromMeta(nodes.posts[77].Meta) == nil {
		t.Errorf("the rest of the meta went: %s", nodes.posts[77].Meta)
	}
}

// One that does not play everywhere is made once, at 720.
func TestAVideoFromElsewhereThatDoesNotPlayIsMadeOnceAt720(t *testing.T) {
	f, copies, nodes := elsewhereFixture(t, "/v.mp4", &encode.Info{Width: 1080, Height: 1920, Duration: 30, VideoCodec: "hevc", PixFmt: "yuv420p"})
	_, _ = f.worker.Once()
	if copies.finished[77][0] != repositories.ElsewhereReady || f.encoder.remuxed != 0 {
		t.Fatalf("finished %v, copied as it is %d times", copies.finished[77], f.encoder.remuxed)
	}
	if video := videos.FromMeta(nodes.posts[77].Meta); video == nil || video.Height != 720 {
		t.Errorf("made at %+v", video)
	}
}

// Gone from its server, or too large, it is said so, and nothing is kept.
func TestAVideoFromElsewhereThatCannotBeHadIsSaidSo(t *testing.T) {
	f, copies, nodes := elsewhereFixture(t, "/gone.mp4", &encode.Info{Width: 640, Height: 360, Duration: 5})
	_, _ = f.worker.Once()
	if copies.finished[77] != [2]string{repositories.ElsewhereFailed, ElsewhereGone} || videos.FromMeta(nodes.posts[77].Meta) != nil {
		t.Errorf("gone: %v", copies.finished[77])
	}

	f, copies, _ = elsewhereFixture(t, "/v.mp4", &encode.Info{Width: 640, Height: 360, Duration: 5})
	f.worker.RemoteMaxBytes = 4
	_, _ = f.worker.Once()
	if copies.finished[77][1] != "too_large" {
		t.Errorf("too large: %v", copies.finished[77])
	}

	f, copies, _ = elsewhereFixture(t, "/v.mp4", &encode.Info{Width: 640, Height: 360, Duration: 5})
	f.worker.Refused = func(string) bool { return true }
	_, _ = f.worker.Once()
	if copies.finished[77] != [2]string{repositories.ElsewhereFailed, ElsewhereRefused} {
		t.Errorf("refused: %v", copies.finished[77])
	}
}

// A copy nobody has played for REMOTE_VIDEO_DAYS is removed, the post left
// leading to where it is.
func TestACopyNobodyPlaysIsRemoved(t *testing.T) {
	f, copies, nodes := elsewhereFixture(t, "/v.mp4", &encode.Info{
		Width: 1280, Height: 720, Duration: 30, VideoCodec: "h264", PixFmt: "yuv420p",
	})
	_, _ = f.worker.Once()
	copies.copies[77].PlayedAt = time.Now().AddDate(0, 0, -8)

	if removed := f.worker.TidyElsewhere(7, time.Now()); removed != 1 {
		t.Fatalf("removed %d", removed)
	}
	if videos.FromMeta(nodes.posts[77].Meta) != nil || videos.ElsewhereFromMeta(nodes.posts[77].Meta) == nil {
		t.Errorf("the post: %s", nodes.posts[77].Meta)
	}
	for _, key := range f.store.keys() {
		if strings.Contains(key, "elsewhere-77") {
			t.Errorf("still stored: %s", key)
		}
	}
	if copies.finished[77][0] != repositories.ElsewhereRemoved {
		t.Errorf("finished %v", copies.finished[77])
	}
}
