package repositories

import (
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Imports against a real database (anahita-web roadmap row 124). Run as
// the exports' tests are: EXPORT_TEST_DSN, a database with
// migration-job/schema/init.sql loaded.

func testImports(t *testing.T) *ImportsRepo {
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
	for _, table := range []string{"data_import_lines", "data_imports"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
	return NewImports(db)
}

func lines(kinds ...string) []*Line {
	out := []*Line{}
	for i, kind := range kinds {
		out = append(out, &Line{Kind: kind, Address: string(rune('a'+i)) + "@rocinante.example", HideNotifications: true})
	}
	return out
}

func tally(t *testing.T, r *ImportsRepo, id string) map[string]int {
	t.Helper()
	counts, err := r.Counts(id)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for _, c := range counts {
		out[c.Kind+" "+c.Status] += c.N
	}
	return out
}

// THE RULE THIS GUARDS: one import at a time for each person; its lines
// are kept as read, those of a kind not chosen are skipped when it is
// started, the rest are taken one at a time and each finished once, and the
// import is done when nothing is left.
func TestAnImportGoesThroughItsLines(t *testing.T) {
	r := testImports(t)

	imp := &Import{ID: uuid.NewString(), PersonID: 5, Source: "mastodon", FileName: "following_accounts.csv"}
	if err := r.Add(imp, lines(KindFollow, KindFollow, KindBlock, KindMute)); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(&Import{ID: uuid.NewString(), PersonID: 5}, lines(KindFollow)); !errors.Is(err, ErrImportActive) {
		t.Fatalf("a second while one waits: %v", err)
	}
	if got := tally(t, r, imp.ID); got["follow pending"] != 2 || got["block pending"] != 1 {
		t.Errorf("kept: %v", got)
	}

	// Nothing is taken before it is started.
	if line, err := r.NextLine(); err != nil || line != nil {
		t.Fatalf("before starting: %v %v", line, err)
	}

	if err := r.Start(imp.ID, []string{KindFollow, KindBlock}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(imp.ID, []string{KindFollow}); !errors.Is(err, ErrImportNotInState) {
		t.Errorf("started twice: %v", err)
	}
	if got := tally(t, r, imp.ID); got["mute skipped"] != 1 {
		t.Errorf("the mute not chosen: %v", got)
	}

	statuses := []string{LineDone, LineFailed, LineAlready}
	for i := 0; ; i++ {
		line, err := r.NextLine()
		if err != nil {
			t.Fatal(err)
		}
		if line == nil {
			break
		}
		if i >= len(statuses) {
			t.Fatalf("a line taken again: %+v", line)
		}
		if err := r.Finish(line.ID, statuses[i], "why"); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.FinishImports(); err != nil {
		t.Fatal(err)
	}
	done, _ := r.Read(imp.ID)
	if done.Status != ImportDone || done.FinishedAt == nil {
		t.Errorf("not done: %+v", done)
	}
	failed, _ := r.Failed(imp.ID)
	if len(failed) != 1 || failed[0].Reason != "why" {
		t.Errorf("failed: %+v", failed)
	}

	// Done, another may be read.
	if err := r.Add(&Import{ID: uuid.NewString(), PersonID: 5}, lines(KindFollow)); err != nil {
		t.Errorf("after one is done: %v", err)
	}
}

// THE RULE THIS GUARDS: a line left working by a worker that stopped is
// taken again; an import given up does no more.
func TestALineLeftBehindIsTakenAgain(t *testing.T) {
	r := testImports(t)
	imp := &Import{ID: uuid.NewString(), PersonID: 6}
	if err := r.Add(imp, lines(KindFollow, KindFollow)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(imp.ID, Kinds); err != nil {
		t.Fatal(err)
	}
	first, _ := r.NextLine()

	// Ten minutes on, the worker that took it never came back.
	r.now = func() time.Time { return time.Now().UTC().Add(11 * time.Minute) }
	again, _ := r.NextLine()
	if again == nil || again.ID != first.ID {
		t.Fatalf("taken again: %+v, want %d", again, first.ID)
	}

	if err := r.Cancel(imp.ID); err != nil {
		t.Fatal(err)
	}
	if line, _ := r.NextLine(); line != nil {
		t.Errorf("after giving up: %+v", line)
	}
	if got := tally(t, r, imp.ID); got["follow skipped"] != 2 {
		t.Errorf("left: %v", got)
	}
}
