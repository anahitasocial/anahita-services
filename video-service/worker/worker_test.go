package worker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/videos"

	"video/constants"
	"video/data/repositories"
	"video/encode"
)

// The worker, with storage, the graph and ffmpeg stood in for. What ffmpeg
// is told is in package encode; what state an upload may go to, in the
// repository's tests against MySQL.

type fakeUploads struct {
	repositories.UploadsInterface
	upload   *repositories.Upload
	claimed  bool
	released int
}

func (f *fakeUploads) Claim() (*repositories.Upload, error) {
	if f.claimed || f.upload.Status != videos.StatusProcessing {
		return nil, nil
	}
	f.claimed = true
	f.upload.Attempts++
	copied := *f.upload
	return &copied, nil
}

func (f *fakeUploads) Read(id string) (*repositories.Upload, error) {
	copied := *f.upload
	return &copied, nil
}

func (f *fakeUploads) ToReady(id string, nodeID int64) error {
	if f.upload.Status != videos.StatusProcessing {
		return repositories.ErrNotInState
	}
	f.upload.Status, f.upload.NodeID = videos.StatusReady, nodeID
	return nil
}

func (f *fakeUploads) ToFailed(id, reason string) error {
	if f.upload.Status != videos.StatusProcessing {
		return repositories.ErrNotInState
	}
	f.upload.Status, f.upload.Error = videos.StatusFailed, reason
	return nil
}

func (f *fakeUploads) Release(id string) error {
	f.released++
	f.claimed = false
	return nil
}

// store is storage: what is kept, by key, behind addresses like the real
// ones.
type store struct {
	mu     sync.Mutex
	kept   map[string][]byte
	types  map[string]string
	server *httptest.Server
	// Refuses every write, as a store that is down would.
	down bool
}

func newStore(t *testing.T) *store {
	s := &store{kept: map[string][]byte{}, types: map[string]string{}}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/")
		s.mu.Lock()
		defer s.mu.Unlock()

		switch r.Method {
		case http.MethodGet:
			body, ok := s.kept[key]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		case http.MethodPut:
			if s.down {
				http.Error(w, "down", http.StatusServiceUnavailable)
				return
			}
			body, _ := io.ReadAll(r.Body)
			s.kept[key] = body
			s.types[key] = r.Header.Get("Content-Type")
		}
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *store) FetchURL(path string) (string, error) { return s.server.URL + "/" + path, nil }
func (s *store) StoreURL(path, contentType string) (string, error) {
	return s.server.URL + "/" + path, nil
}
func (s *store) UploadStart(path string) (string, error) { return "", nil }
func (s *store) PresignDownload(path, name string) (string, error) {
	return s.server.URL + "/" + path, nil
}
func (s *store) UploadParts(path, id string, n []int32) ([]string, error) {
	return nil, nil
}
func (s *store) UploadComplete(path, id string, max int64) (int64, error) { return 0, nil }
func (s *store) UploadAbort(path, id string) error                        { return nil }
func (s *store) Stat(path string) (bool, int64, error)                    { return false, 0, nil }

func (s *store) Delete(req *services.FileDeleteRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.kept, req.Path)
	return nil
}

func (s *store) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []string
	for key := range s.kept {
		keys = append(keys, key)
	}
	return keys
}

type fakeEncoder struct {
	info     *encode.Info
	probeErr error
	// Called while the sizes are being made.
	during func()
}

func (f *fakeEncoder) Probe(ctx context.Context, path string, max int) (*encode.Info, string, error) {
	if f.probeErr != nil {
		return nil, "", f.probeErr
	}
	return f.info, "mov", nil
}

func (f *fakeEncoder) Encode(ctx context.Context, container, in, out string, target encode.Target, max int) error {
	if f.during != nil {
		f.during()
	}
	return os.WriteFile(out, []byte("encoded video"), 0o644)
}

func (f *fakeEncoder) Poster(ctx context.Context, container, in, out string, info *encode.Info) error {
	return os.WriteFile(out, []byte("a still"), 0o644)
}

type fakePosts struct {
	added   []*entities.AnahitaNode
	deleted []int64
}

func (f *fakePosts) Add(item *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	if viewer.ID != item.CreatedBy {
		return nil, errors.New("posted as somebody else")
	}
	item.ID = 77
	f.added = append(f.added, item)
	return item, nil
}

func (f *fakePosts) Delete(id int64, viewer *entities.Viewer) error {
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeOwners struct{}

func (fakeOwners) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	owner := &entities.AnahitaNode{}
	owner.ID = id
	owner.Type = "node.actor.group-service.group.v1"
	return owner, nil
}

type fixture struct {
	worker  *Worker
	uploads *fakeUploads
	store   *store
	posts   *fakePosts
	encoder *fakeEncoder
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	upload := &repositories.Upload{
		ID: "0b8f6a52-1f0e-4a7e-9a52-0a6f6f1f0e4a", PersonID: 5, OwnerID: 9,
		Name: "Sunset from the bridge", Body: "West End", Access: "followers",
		Language: "en", ReplyAccess: "nobody", QuotePolicy: "followers",
		Status: videos.StatusProcessing,
	}
	upload.SourceKey = videos.SourceKey(5, upload.ID)

	f := &fixture{
		uploads: &fakeUploads{upload: upload},
		store:   newStore(t),
		posts:   &fakePosts{},
		encoder: &fakeEncoder{info: &encode.Info{Width: 1920, Height: 1080, Duration: 12.5}},
	}
	f.store.kept[upload.SourceKey] = []byte("what was uploaded")
	f.worker = &Worker{
		Uploads:    f.uploads,
		Transfers:  f.store,
		Files:      f.store,
		Posts:      f.posts,
		Owners:     fakeOwners{},
		Encoder:    f.encoder,
		Dir:        t.TempDir(),
		Heights:    []int{1080, 720, 360},
		MaxBytes:   1 << 20,
		MaxSeconds: 600,
		Timeout:    time.Minute,
	}
	return f
}

func TestAnUploadBecomesAPost(t *testing.T) {
	f := newFixture(t)

	worked, err := f.worker.Once()
	if err != nil || !worked {
		t.Fatalf("worked %v %v", worked, err)
	}

	if f.uploads.upload.Status != videos.StatusReady || f.uploads.upload.NodeID != 77 {
		t.Fatalf("the upload ended as %+v", f.uploads.upload)
	}

	// Stored: each size and the still, as what they are. The upload is gone.
	folder := videos.Folder(9, f.uploads.upload.ID)
	for key, kind := range map[string]string{
		folder + "1080.mp4":   "video/mp4",
		folder + "720.mp4":    "video/mp4",
		folder + "360.mp4":    "video/mp4",
		folder + "poster.jpg": "image/jpeg",
	} {
		if _, ok := f.store.kept[key]; !ok || f.store.types[key] != kind {
			t.Errorf("%s: kept %v as %q", key, ok, f.store.types[key])
		}
	}
	if _, kept := f.store.kept[f.uploads.upload.SourceKey]; kept || len(f.store.keys()) != 4 {
		t.Errorf("left in storage: %v", f.store.keys())
	}

	// The post: as its uploader, on the profile and with everything they
	// chose, saying where its files are.
	if len(f.posts.added) != 1 {
		t.Fatalf("%d posts made", len(f.posts.added))
	}
	post := f.posts.added[0]
	if post.Type != constants.NodeMediumVideoType || post.CreatedBy != 5 || post.OwnerID != 9 ||
		post.Name != "Sunset from the bridge" || post.Body != "West End" || post.Access != "followers" ||
		post.Language != "en" || post.ReplyAccess != "nobody" || post.CommentStatus || post.QuotePolicy != "followers" ||
		!post.Enabled || post.Filename != "" {
		t.Errorf("posted as %+v", post)
	}
	video := videos.FromMeta(post.Meta)
	if video == nil || len(video.Renditions) != 3 || video.Renditions[0].Height != 1080 || video.Renditions[0].Width != 1920 ||
		video.Poster != folder+"poster.jpg" || video.Duration != 12.5 || video.Width != 1920 {
		t.Errorf("the post says its video is %+v", video)
	}

	// Nothing more to do.
	if worked, _ := f.worker.Once(); worked {
		t.Errorf("worked again")
	}
}

func TestWhatIsNotAVideoIsRefusedForGood(t *testing.T) {
	for reason, probeErr := range map[string]error{
		constants.FailedNotAVideo: encode.ErrNotAVideo,
		constants.FailedTooLong:   encode.ErrTooLong,
	} {
		f := newFixture(t)
		f.encoder.probeErr = probeErr

		f.worker.Once()

		if f.uploads.upload.Status != videos.StatusFailed || f.uploads.upload.Error != reason {
			t.Errorf("%s: ended as %+v", reason, f.uploads.upload)
		}
		if len(f.posts.added) != 0 || len(f.store.keys()) != 0 || f.uploads.released != 0 {
			t.Errorf("%s: posts %d, kept %v, let go %d", reason, len(f.posts.added), f.store.keys(), f.uploads.released)
		}
	}

	// Larger than was said when it was finished.
	f := newFixture(t)
	f.worker.MaxBytes = 4
	f.worker.Once()
	if f.uploads.upload.Error != constants.FailedTooLarge || len(f.store.keys()) != 0 {
		t.Errorf("a file past the limit: %+v, kept %v", f.uploads.upload, f.store.keys())
	}
}

func TestATroubleThatMayPassIsTriedAgainAndThenGivenUp(t *testing.T) {
	f := newFixture(t)
	f.store.down = true

	for attempt := 1; attempt <= constants.MaxAttempts; attempt++ {
		if worked, _ := f.worker.Once(); !worked {
			t.Fatalf("attempt %d: nothing taken", attempt)
		}
		if attempt < constants.MaxAttempts {
			// Let go of, with the upload kept for the next try.
			if f.uploads.upload.Status != videos.StatusProcessing || f.uploads.released != attempt {
				t.Fatalf("attempt %d: %+v, let go %d", attempt, f.uploads.upload, f.uploads.released)
			}
			if _, kept := f.store.kept[f.uploads.upload.SourceKey]; !kept {
				t.Fatalf("attempt %d: the upload was thrown away", attempt)
			}
		}
	}

	if f.uploads.upload.Status != videos.StatusFailed || f.uploads.upload.Error != constants.FailedEncoding {
		t.Errorf("ended as %+v", f.uploads.upload)
	}
	if len(f.posts.added) != 0 || len(f.store.keys()) != 0 {
		t.Errorf("posts %d, kept %v", len(f.posts.added), f.store.keys())
	}
}

func TestAVideoGivenUpWhileItIsMadeIsNotPosted(t *testing.T) {
	f := newFixture(t)
	f.encoder.during = func() { f.uploads.upload.Status = videos.StatusCancelled }

	f.worker.Once()

	if len(f.posts.added) != 0 {
		t.Errorf("a post was made of a video that was given up")
	}
	if f.uploads.upload.Status != videos.StatusCancelled {
		t.Errorf("ended as %+v", f.uploads.upload)
	}
	// Nothing made of it is left.
	for _, key := range f.store.keys() {
		if strings.HasPrefix(key, videos.StoragePrefix) {
			t.Errorf("left in storage: %s", key)
		}
	}
}

func TestTheTimeAllowedGrowsWithLengthAndSize(t *testing.T) {
	usual := []int{1080, 720, 360}
	if got := TimeoutFor(600, usual); got != 52*time.Minute+30*time.Second {
		t.Errorf("ten minutes at the usual sizes: %v", got)
	}
	if got := TimeoutFor(60, usual); got != 30*time.Minute {
		t.Errorf("a short limit: %v", got)
	}
	// 4K as well: a great deal longer.
	if got := TimeoutFor(600, []int{2160, 1080, 720, 360}); got < 3*time.Hour {
		t.Errorf("ten minutes with 4K: %v", got)
	}
	if got := TimeoutFor(3600, usual); got < 5*time.Hour {
		t.Errorf("an hour of video: %v", got)
	}
}
