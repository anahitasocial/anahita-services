package federation

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Skipped unless FEDERATION_TEST_DSN names a throwaway MySQL: it makes and
// drops a table of its own, and is never to be pointed at a database
// anybody uses.
func testQueue(t *testing.T) (*Queue, *sql.DB) {
	t.Helper()

	dsn := os.Getenv("FEDERATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set FEDERATION_TEST_DSN to run the delivery queue's tests against MySQL")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	const table = "test_deliveries"
	if _, err := db.Exec(`DROP TABLE IF EXISTS ` + table); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(TableSQL(table)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(`DROP TABLE IF EXISTS ` + table) })

	queue, err := NewQueue(db, table)
	if err != nil {
		t.Fatal(err)
	}
	return queue, db
}

// THE RULE THIS GUARDS: what is to be sent is sent once, by one worker, and
// is neither lost when sending fails nor tried for ever.
func TestWhatIsQueuedIsSentOnce(t *testing.T) {
	queue, db := testQueue(t)
	ctx := context.Background()

	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	queue.now = func() time.Time { return clock }

	count := func(status string) (n int) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM test_deliveries WHERE status = ?`, status).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Added once, however often the same thing is added.
	for i := 0; i < 3; i++ {
		if err := queue.Enqueue(ctx, 1, "https://mastodon.example/inbox", []byte(`{"id":"a"}`), "a>mastodon.example"); err != nil {
			t.Fatal(err)
		}
	}
	if err := queue.Enqueue(ctx, 1, "https://other.example/inbox", []byte(`{"id":"a"}`), "a>other.example"); err != nil {
		t.Fatal(err)
	}
	if got := count(statusPending); got != 2 {
		t.Fatalf("%d waiting, want 2", got)
	}
	if err := queue.Enqueue(ctx, 0, "", nil, ""); err == nil {
		t.Error("nothing was queued for nobody")
	}

	// Taken by one caller, it is not there for the next.
	first, err := queue.Claim(ctx, 1)
	if err != nil || len(first) != 1 || first[0].Target != "https://mastodon.example/inbox" || string(first[0].Payload) != `{"id":"a"}` {
		t.Fatalf("first took %+v (%v)", first, err)
	}
	second, err := queue.Claim(ctx, 10)
	if err != nil || len(second) != 1 || second[0].ID == first[0].ID {
		t.Fatalf("second took %+v (%v)", second, err)
	}
	if third, _ := queue.Claim(ctx, 10); len(third) != 0 {
		t.Fatalf("a third took %+v", third)
	}

	// Sent, it is gone.
	if err := queue.Done(ctx, first[0].ID); err != nil {
		t.Fatal(err)
	}

	// Failed, it waits, each time longer, and is not due before then.
	failed := second[0]
	for attempt, wait := range []time.Duration{time.Minute, 4 * time.Minute, 16 * time.Minute, 64 * time.Minute, 256 * time.Minute, 6 * time.Hour, 6 * time.Hour} {
		if err := queue.Fail(ctx, failed, errors.New("503 from the other server")); err != nil {
			t.Fatal(err)
		}

		clock = clock.Add(wait - time.Second)
		if early, _ := queue.Claim(ctx, 10); len(early) != 0 {
			t.Fatalf("after %d failures it was due %v early", attempt+1, time.Second)
		}
		clock = clock.Add(time.Second)
		due, err := queue.Claim(ctx, 10)
		if err != nil || len(due) != 1 || due[0].Attempts != attempt+1 {
			t.Fatalf("after %d failures: %+v (%v)", attempt+1, due, err)
		}
		failed = due[0]
	}

	// Two days on, it is given up, and kept to be looked into.
	clock = clock.Add(48 * time.Hour)
	if err := queue.Fail(ctx, failed, errors.New("still 503")); err != nil {
		t.Fatal(err)
	}
	if got := count(statusFailed); got != 1 {
		t.Errorf("%d given up, want 1", got)
	}
	if again, _ := queue.Claim(ctx, 10); len(again) != 0 {
		t.Errorf("what was given up was tried again: %+v", again)
	}
	var reason string
	if err := db.QueryRow(`SELECT last_error FROM test_deliveries WHERE id = ?`, failed.ID).Scan(&reason); err != nil || reason != "still 503" {
		t.Errorf("why it failed was kept as %q (%v)", reason, err)
	}

	// What trying again will not mend is given up at once.
	if err := queue.Enqueue(ctx, 1, "https://gone.example/inbox", []byte(`{}`), ""); err != nil {
		t.Fatal(err)
	}
	gone, _ := queue.Claim(ctx, 10)
	if len(gone) != 1 {
		t.Fatalf("took %+v", gone)
	}
	if err := queue.Fail(ctx, gone[0], Permanent(errors.New("410 Gone"))); err != nil {
		t.Fatal(err)
	}
	if got := count(statusFailed); got != 2 {
		t.Errorf("%d given up, want 2", got)
	}

	// A worker that took something and stopped does not keep it for ever.
	if err := queue.Enqueue(ctx, 2, "https://mastodon.example/inbox", []byte(`{"id":"b"}`), ""); err != nil {
		t.Fatal(err)
	}
	held, _ := queue.Claim(ctx, 10)
	if len(held) != 1 {
		t.Fatalf("took %+v", held)
	}
	clock = clock.Add(sendingTimeout - time.Second)
	if early, _ := queue.Claim(ctx, 10); len(early) != 0 {
		t.Error("what a worker holds was handed to another too soon")
	}
	clock = clock.Add(2 * time.Second)
	if again, _ := queue.Claim(ctx, 10); len(again) != 1 || again[0].ID != held[0].ID {
		t.Errorf("what a stopped worker held was not handed out again: %+v", again)
	}
}

// Many workers on one table: everything is sent, and nothing twice.
func TestWorkersDoNotShareARow(t *testing.T) {
	queue, db := testQueue(t)
	ctx := context.Background()

	const things = 200
	for i := 0; i < things; i++ {
		if err := queue.Enqueue(ctx, 1, "https://mastodon.example/inbox", []byte{byte(i)}, ""); err != nil {
			t.Fatal(err)
		}
	}

	var (
		mu   sync.Mutex
		sent = map[int64]int{}
		wait sync.WaitGroup
	)
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for {
				claimed, err := queue.Claim(ctx, 7)
				if err != nil {
					t.Error(err)
					return
				}
				if len(claimed) == 0 {
					// Nothing free this moment is not nothing left: what
					// another worker is taking is passed over, not waited
					// for.
					var left int
					if err := db.QueryRow(`SELECT COUNT(*) FROM test_deliveries`).Scan(&left); err != nil || left == 0 {
						return
					}
					continue
				}
				for _, d := range claimed {
					mu.Lock()
					sent[d.ID]++
					mu.Unlock()
					if err := queue.Done(ctx, d.ID); err != nil {
						t.Error(err)
					}
				}
			}
		}()
	}
	wait.Wait()

	if len(sent) != things {
		t.Errorf("%d sent, want %d", len(sent), things)
	}
	for id, times := range sent {
		if times != 1 {
			t.Errorf("delivery %d was sent %d times", id, times)
		}
	}
}

func TestRunSendsUntilToldToStop(t *testing.T) {
	queue, db := testQueue(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	for _, target := range []string{"https://a.example/inbox", "https://b.example/inbox", "https://gone.example/inbox"} {
		if err := queue.Enqueue(ctx, 1, target, []byte(`{}`), ""); err != nil {
			t.Fatal(err)
		}
	}

	var (
		mu   sync.Mutex
		seen []string
	)
	done := make(chan struct{})
	go func() {
		queue.Run(ctx, 20*time.Millisecond, 2, func(_ context.Context, d Delivery) error {
			mu.Lock()
			seen = append(seen, d.Target)
			mu.Unlock()
			if d.Target == "https://gone.example/inbox" {
				return Permanent(errors.New("410"))
			}
			return nil
		})
		close(done)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		var pending int
		if err := db.QueryRow(`SELECT COUNT(*) FROM test_deliveries WHERE status <> ?`, statusFailed).Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if pending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d still waiting", pending)
		}
		time.Sleep(20 * time.Millisecond)
	}

	stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("it did not stop when told to")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 3 {
		t.Errorf("sent %v", seen)
	}
}

func TestATableIsNamedAsOne(t *testing.T) {
	for _, bad := range []string{"", "deliveries; DROP TABLE nodes", "Deliveries", "a b", "1st", "nodes`"} {
		if _, err := NewQueue(nil, bad); err == nil {
			t.Errorf("%q was taken for a table's name", bad)
		}
	}
	if _, err := NewQueue(nil, "activitypub_deliveries"); err != nil {
		t.Error(err)
	}
}
