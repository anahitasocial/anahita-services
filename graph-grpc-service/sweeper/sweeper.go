// Package sweeper makes account deletion permanent.
//
// Deleting a profile marks it and disables it; this is what eventually destroys
// it. Without it `deleted_at` would be written and never read, and deleted
// accounts would sit in the database forever — a broken promise to the person
// who asked to leave, and a problem under any erasure obligation.
//
// It is a JOB, not a background goroutine inside the gRPC server, and that is
// deliberate. graph-grpc-service runs three replicas in Kubernetes
// (k8s/03-graph-grpc.yml), so an in-process loop would run three times at once:
// three replicas reading the same queue and issuing overlapping recursive
// deletes against the same rows. Not corrupting, but a real source of InnoDB
// deadlocks, triplicate events, and logs that contradict each other.
//
// A single-run job also earns things a goroutine cannot offer: it can be
// triggered by hand, its exit code says whether it worked, and its schedule
// lives with the rest of the deployment rather than inside a service binary.
//
// The destructive work is split across services along the same line as the rest
// of deletion. This purges the content graph and tombstones the actor; the
// `purged` event it publishes is what tells auth-service to remove the
// credentials, auth logs and pending tokens it owns. Neither service reaches
// into the other's tables.
package sweeper

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"anahita-libs/data/services"
	"anahita-libs/event"
	"graph/data/repositories"
	"graph/purge"
)

const (
	// BatchSize caps one run.
	//
	// Each purge is a recursive delete across nodes and edges, so an unbounded
	// run against a backlog could hold locks long enough to look like an
	// outage. A backlog drains over successive runs instead; at hourly
	// scheduling that is 2,400 accounts a day, far past any realistic rate.
	BatchSize = 100

	// GraceDays must match auth-service's constants.AccountGraceDays, which is
	// the date the restore page promises the person.
	//
	// Duplicated rather than shared because the two services have no common
	// constants package, and importing auth's would point a data service at an
	// auth service. If one moves, move both — a sweeper that runs early erases
	// accounts inside the window somebody was told they had.
	GraceDays = 30

	// lockName is a MySQL advisory lock held for the duration of a run.
	//
	// Belt and braces on top of running as a single-instance job: a slow run
	// overlapping the next scheduled one, a manual invocation racing the cron,
	// or somebody reinstating an in-process caller would all otherwise put two
	// sweeps on the same queue.
	lockName = "anahita_account_sweeper"

	// lockTimeout is how long to wait for the lock before giving up.
	//
	// Zero: if another sweep holds it, this run has nothing to add, and the
	// next tick comes soon enough. Waiting would only stack up runs behind a
	// slow one.
	lockTimeout = 0
)

// Run purges every account whose grace period has expired, up to BatchSize.
//
// Returns the number purged. An error means the run could not proceed at all;
// individual accounts that fail are logged and skipped, because one bad row
// should not block every account queued behind it.
func Run(db *sql.DB, repos *repositories.Repositories, publisher *event.Publisher, files services.FilesGRPCInterface) (int, error) {
	acquired, release, err := acquireLock(db)
	if err != nil {
		return 0, fmt.Errorf("acquiring the sweeper lock: %w", err)
	}
	if !acquired {
		log.Println("sweeper: another run holds the lock, skipping")
		return 0, nil
	}
	defer release()

	cutoff := time.Now().UTC().AddDate(0, 0, -GraceDays)

	ids, err := repos.Nodes.BrowseDeletedBefore(cutoff, BatchSize)
	if err != nil {
		return 0, fmt.Errorf("reading the purge queue: %w", err)
	}

	if len(ids) == 0 {
		log.Println("sweeper: nothing past the grace period")
		return 0, nil
	}

	log.Printf("sweeper: %d account(s) past the grace period", len(ids))

	purged := 0
	for _, id := range ids {
		if err := purge.Account(repos, publisher, files, id); err != nil {
			log.Printf("sweeper: purging %d failed: %v", id, err)
			continue
		}
		purged++
	}

	log.Printf("sweeper: purged %d of %d", purged, len(ids))

	return purged, nil
}

// acquireLock takes a MySQL advisory lock, returning whether it was obtained
// and a function to release it.
//
// GET_LOCK is session-scoped, so the lock must be taken and released on ONE
// connection — hence the dedicated conn rather than the pool, where the release
// could otherwise land on a different connection and silently do nothing.
//
// The lock also disappears if the process dies, which is the behaviour worth
// having: a crashed sweep should not block every future one.
func acquireLock(db *sql.DB) (bool, func(), error) {
	ctx := context.Background()

	conn, err := db.Conn(ctx)
	if err != nil {
		return false, nil, err
	}

	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx,
		`SELECT GET_LOCK(?, ?)`, lockName, lockTimeout).Scan(&got); err != nil {
		_ = conn.Close()
		return false, nil, err
	}

	// NULL means the attempt itself errored; treat it as not acquired rather
	// than sweeping without a lock.
	if !got.Valid || got.Int64 != 1 {
		_ = conn.Close()
		return false, nil, nil
	}

	release := func() {
		if _, err := conn.ExecContext(ctx, `SELECT RELEASE_LOCK(?)`, lockName); err != nil {
			log.Println("sweeper: releasing the lock failed:", err)
		}
		_ = conn.Close()
	}

	return true, release, nil
}
