package repositories

import (
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"anahita-libs/videos"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// Against a real MySQL, on a throwaway database with the schema loaded:
//
//	docker run -d --name anahita-acl-test -e MYSQL_ALLOW_EMPTY_PASSWORD=1 \
//	  -e MYSQL_DATABASE=fresh -p 13309:3306 mysql:9.6
//	docker exec -i anahita-acl-test mysql -uroot fresh < migration-job/schema/init.sql
//	VIDEO_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" go test ./data/repositories/
//
// THE RULE THIS GUARDS: an upload is in one state, and two things changing
// it at once cannot both succeed. Two workers do not make the same video,
// and one given up is not made a post.

func testRepo(t *testing.T) *UploadsRepo {
	t.Helper()

	dsn := os.Getenv("VIDEO_TEST_DSN")
	if dsn == "" {
		t.Skip("set VIDEO_TEST_DSN to run against MySQL")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("DELETE FROM video_uploads"); err != nil {
		t.Fatal(err)
	}
	return NewUploads(db)
}

func add(t *testing.T, r *UploadsRepo, person int64) *Upload {
	t.Helper()
	upload := &Upload{
		ID: uuid.NewString(), PersonID: person, OwnerID: person, Name: "Sunset",
		Body: "From the bridge", Access: "public", DeclaredBytes: 1000,
		StorageUploadID: "store-1",
	}
	upload.SourceKey = videos.SourceKey(person, upload.ID)
	if err := r.Add(upload); err != nil {
		t.Fatal(err)
	}
	return upload
}

func TestAnUploadGoesThroughItsStatesOneWayOnly(t *testing.T) {
	r := testRepo(t)
	upload := add(t, r, 5)

	read, err := r.Read(upload.ID)
	if err != nil || read == nil || read.Status != videos.StatusUploading || read.Name != "Sunset" || read.Body != "From the bridge" {
		t.Fatalf("read back %+v %v", read, err)
	}
	if missing, err := r.Read(uuid.NewString()); err != nil || missing != nil {
		t.Errorf("an upload that is not there: %+v %v", missing, err)
	}

	// Not ready before it is all there, and nothing to take.
	if err := r.ToReady(upload.ID, 9); !errors.Is(err, ErrNotInState) {
		t.Errorf("ready while uploading: %v", err)
	}
	if taken, _ := r.Claim(); taken != nil {
		t.Errorf("an upload still being sent was taken: %+v", taken)
	}

	if err := r.ToProcessing(upload.ID, 990); err != nil {
		t.Fatal(err)
	}
	if err := r.ToProcessing(upload.ID, 990); !errors.Is(err, ErrNotInState) {
		t.Errorf("completed twice: %v", err)
	}

	taken, err := r.Claim()
	if err != nil || taken == nil || taken.ID != upload.ID || taken.Attempts != 1 || taken.SizeBytes != 990 || taken.StorageUploadID != "" {
		t.Fatalf("taken %+v %v", taken, err)
	}
	// Held: not taken again while its worker may still be alive.
	if again, _ := r.Claim(); again != nil {
		t.Errorf("taken twice: %+v", again)
	}

	if err := r.ToReady(upload.ID, 42); err != nil {
		t.Fatal(err)
	}
	read, _ = r.Read(upload.ID)
	if read.Status != videos.StatusReady || read.NodeID != 42 {
		t.Errorf("ended as %+v", read)
	}
	// Finished is finished.
	for name, err := range map[string]error{
		"failed":    r.ToFailed(upload.ID, "encoding"),
		"cancelled": r.ToCancelled(upload.ID),
		"ready":     r.ToReady(upload.ID, 43),
	} {
		if !errors.Is(err, ErrNotInState) {
			t.Errorf("a finished upload was %s: %v", name, err)
		}
	}
}

func TestAVideoGivenUpIsNotMadeAPost(t *testing.T) {
	r := testRepo(t)
	upload := add(t, r, 5)
	_ = r.ToProcessing(upload.ID, 990)
	if taken, _ := r.Claim(); taken == nil {
		t.Fatal("not taken")
	}

	// Its uploader gives it up while the worker has it.
	if err := r.ToCancelled(upload.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.ToReady(upload.ID, 42); !errors.Is(err, ErrNotInState) {
		t.Errorf("made a post after it was given up: %v", err)
	}
}

func TestTwoWorkersDoNotTakeTheSameVideo(t *testing.T) {
	r := testRepo(t)
	for i := 0; i < 3; i++ {
		upload := add(t, r, int64(10+i))
		_ = r.ToProcessing(upload.ID, 990)
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		taken = map[string]int{}
	)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			upload, err := r.Claim()
			if err != nil || upload == nil {
				return
			}
			mu.Lock()
			taken[upload.ID]++
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(taken) != 3 {
		t.Errorf("%d of 3 were taken", len(taken))
	}
	for id, times := range taken {
		if times != 1 {
			t.Errorf("%s was taken %d times", id, times)
		}
	}
}

func TestAVideoLeftByADeadWorkerIsTakenAgainAndThenGivenUp(t *testing.T) {
	r := testRepo(t)
	upload := add(t, r, 5)
	_ = r.ToProcessing(upload.ID, 990)

	now := time.Now().UTC()
	r.now = func() time.Time { return now }

	for attempt := 1; attempt <= 3; attempt++ {
		taken, err := r.Claim()
		if err != nil || taken == nil || taken.Attempts != attempt {
			t.Fatalf("attempt %d: %+v %v", attempt, taken, err)
		}
		// Its worker dies. Nothing takes it until that is certain.
		if early, _ := r.Claim(); early != nil {
			t.Fatalf("taken while held")
		}
		now = now.Add(r.ClaimFor + time.Minute)
	}

	// Tried as often as it will be.
	if again, _ := r.Claim(); again != nil {
		t.Errorf("taken a fourth time: %+v", again)
	}
	worn, err := r.WornOut()
	if err != nil || len(worn) != 1 || worn[0] != upload.ID {
		t.Errorf("worn out: %v %v", worn, err)
	}

	// One let go of by its worker is taken again at once.
	other := add(t, r, 6)
	_ = r.ToProcessing(other.ID, 990)
	if taken, _ := r.Claim(); taken == nil || taken.ID != other.ID {
		t.Fatalf("not taken: %+v", taken)
	}
	if err := r.Release(other.ID); err != nil {
		t.Fatal(err)
	}
	if taken, _ := r.Claim(); taken == nil || taken.ID != other.ID || taken.Attempts != 2 {
		t.Errorf("after being let go: %+v", taken)
	}
}

func TestWhatSomebodyHasInHandAndWhatIsAbandoned(t *testing.T) {
	r := testRepo(t)
	now := time.Now().UTC()
	r.now = func() time.Time { return now }

	sending := add(t, r, 5)
	making := add(t, r, 5)
	_ = r.ToProcessing(making.ID, 1)
	failed := add(t, r, 5)
	_ = r.ToFailed(failed.ID, "not_a_video")
	done := add(t, r, 5)
	_ = r.ToProcessing(done.ID, 1)
	_ = r.ToReady(done.ID, 42)
	add(t, r, 6)

	mine, err := r.OfPerson(5)
	if err != nil || len(mine) != 3 {
		t.Fatalf("in hand: %d %v", len(mine), err)
	}
	for _, upload := range mine {
		if upload.ID == done.ID || upload.PersonID != 5 {
			t.Errorf("listed %+v", upload)
		}
	}
	if count, _ := r.CountSince(5, now.Add(-time.Hour)); count != 4 {
		t.Errorf("counted %d sent today", count)
	}

	// A day and more later: the one never finished is abandoned, the
	// failure is no longer shown.
	now = now.Add(25 * time.Hour)
	abandoned, err := r.Abandoned()
	if err != nil || len(abandoned) != 2 {
		t.Fatalf("abandoned: %d %v", len(abandoned), err)
	}
	found := false
	for _, upload := range abandoned {
		found = found || upload.ID == sending.ID
	}
	if !found {
		t.Errorf("the upload never finished is not among the abandoned")
	}
	if mine, _ := r.OfPerson(5); len(mine) != 2 {
		t.Errorf("a day on, %d in hand", len(mine))
	}

	if err := r.Forget(now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if gone, _ := r.Read(done.ID); gone != nil {
		t.Errorf("a settled upload was not forgotten")
	}
	if kept, _ := r.Read(making.ID); kept == nil {
		t.Errorf("one still being made was forgotten")
	}
}
