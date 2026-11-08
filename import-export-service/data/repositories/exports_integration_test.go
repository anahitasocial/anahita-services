package repositories

import (
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// Against a real MySQL, on a throwaway database with the schema loaded:
//
//	EXPORT_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" go test ./data/repositories/
//
// THE RULES THIS GUARDS: one export at a time for each person, whoever asks
// first, by the table and not by the code; an export given up is never
// marked ready; and two workers do not make the same one.

func testRepo(t *testing.T) *ExportsRepo {
	t.Helper()

	dsn := os.Getenv("EXPORT_TEST_DSN")
	if dsn == "" {
		t.Skip("set EXPORT_TEST_DSN to run against MySQL")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("DELETE FROM data_exports"); err != nil {
		t.Fatal(err)
	}
	return NewExports(db)
}

func ask(t *testing.T, r *ExportsRepo, person int64) *Export {
	t.Helper()
	export := &Export{ID: uuid.NewString(), PersonID: person, IP: "203.0.113.9", UserAgent: "test"}
	if err := r.Add(export); err != nil {
		t.Fatal(err)
	}
	return export
}

func TestOneExportAtATimeForEachPerson(t *testing.T) {
	r := testRepo(t)
	first := ask(t, r, 5)

	again := &Export{ID: uuid.NewString(), PersonID: 5}
	if err := r.Add(again); !errors.Is(err, ErrActive) {
		t.Fatalf("a second while one is waiting: %v", err)
	}
	// Somebody else is not held up by it.
	ask(t, r, 6)

	// Still one at a time while it is being made.
	if taken, err := r.Claim(); err != nil || taken == nil || taken.ID != first.ID || taken.Status != StatusBuilding || taken.Attempts != 1 {
		t.Fatalf("taken %+v %v", taken, err)
	}
	if err := r.Add(again); !errors.Is(err, ErrActive) {
		t.Fatalf("a second while one is being made: %v", err)
	}

	// Once it is done, another can be asked for.
	if err := r.ToReady(first.ID, "exports/5/x.zip", 1234, 7*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(again); err != nil {
		t.Errorf("another after the first was ready: %v", err)
	}

	read, _ := r.Read(first.ID)
	if read.Status != StatusReady || read.SizeBytes != 1234 || read.PackageKey != "exports/5/x.zip" || read.ReadyAt == nil || read.ExpiresAt == nil {
		t.Fatalf("read back %+v", read)
	}
	if kept := read.ExpiresAt.Sub(*read.ReadyAt); kept != 7*24*time.Hour {
		t.Errorf("kept for %v", kept)
	}
	if mine, _ := r.OfPerson(5); len(mine) != 2 {
		t.Errorf("%d listed", len(mine))
	}
	if missing, err := r.Read(uuid.NewString()); err != nil || missing != nil {
		t.Errorf("an export that is not there: %+v %v", missing, err)
	}
}

func TestAnExportGivenUpIsNeverMadeReady(t *testing.T) {
	r := testRepo(t)
	export := ask(t, r, 5)
	if taken, _ := r.Claim(); taken == nil {
		t.Fatal("not taken")
	}

	if err := r.ToCancelled(export.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.ToReady(export.ID, "exports/5/x.zip", 1, time.Hour); !errors.Is(err, ErrNotInState) {
		t.Errorf("made ready after it was given up: %v", err)
	}
	// Given up, it does not count against asking again.
	if last, _ := r.LastMade(5); !last.IsZero() {
		t.Errorf("an export given up counts as made: %v", last)
	}
}

func TestWhenSomebodyLastHadOneMade(t *testing.T) {
	r := testRepo(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }

	if last, err := r.LastMade(5); err != nil || !last.IsZero() {
		t.Fatalf("never asked: %v %v", last, err)
	}

	failed := ask(t, r, 5)
	_ = r.ToFailed(failed.ID, "failed")
	if last, _ := r.LastMade(5); !last.IsZero() {
		t.Errorf("a failure counts as made: %v", last)
	}

	now = now.Add(time.Hour)
	made := ask(t, r, 5)
	_, _ = r.Claim()
	_ = r.ToReady(made.ID, "exports/5/y.zip", 1, time.Hour)
	if last, _ := r.LastMade(5); !last.Equal(now) {
		t.Errorf("last made %v, want %v", last, now)
	}

	// Past its keeping, deleted, and still when it was last made.
	now = now.Add(2 * time.Hour)
	past, err := r.PastKeeping()
	if err != nil || len(past) != 1 || past[0].ID != made.ID {
		t.Fatalf("past keeping: %v %v", past, err)
	}
	if err := r.ToExpired(made.ID); err != nil {
		t.Fatal(err)
	}
	if last, _ := r.LastMade(5); last.IsZero() {
		t.Errorf("an expired export no longer counts as made")
	}
}

func TestAnExportLeftByADeadWorkerIsTakenAgainAndThenGivenUp(t *testing.T) {
	r := testRepo(t)
	export := ask(t, r, 5)
	now := time.Now().UTC()
	r.now = func() time.Time { return now }

	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		taken, err := r.Claim()
		if err != nil || taken == nil || taken.Attempts != attempt {
			t.Fatalf("attempt %d: %+v %v", attempt, taken, err)
		}
		if early, _ := r.Claim(); early != nil {
			t.Fatalf("taken while held")
		}
		now = now.Add(r.ClaimFor + time.Minute)
	}

	if again, _ := r.Claim(); again != nil {
		t.Errorf("taken once more than it will be tried")
	}
	worn, err := r.WornOut()
	if err != nil || len(worn) != 1 || worn[0] != export.ID {
		t.Errorf("worn out: %v %v", worn, err)
	}

	// One let go of is taken again at once.
	other := ask(t, r, 6)
	if taken, _ := r.Claim(); taken == nil || taken.ID != other.ID {
		t.Fatalf("not taken")
	}
	if err := r.Release(other.ID); err != nil {
		t.Fatal(err)
	}
	if taken, _ := r.Claim(); taken == nil || taken.ID != other.ID || taken.Attempts != 2 {
		t.Errorf("after being let go: %+v", taken)
	}
}
