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
	"strings"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"graph/data/repositories"
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
		if err := purgeAccount(repos, publisher, files, id); err != nil {
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

// purgeAccount destroys one account's content and tombstones it.
//
// Ordering matters. The content goes first and the tombstone second, so an
// interruption between them leaves an account still marked deleted and still in
// the queue — it gets picked up next run and finishes. The reverse order would
// produce a tombstone with a live content graph hanging off it and nothing left
// to find that content by.
func purgeAccount(repos *repositories.Repositories, publisher *event.Publisher, files services.FilesGRPCInterface, id int64) error {
	// Read before destroying: the event needs the type, and once the content is
	// gone this is the only chance to learn anything about what was purged.
	node, err := repos.Nodes.Read(id)
	if err != nil {
		return fmt.Errorf("reading node: %w", err)
	}

	// Blobs BEFORE the rows. Once the nodes are gone nothing records which
	// objects belonged to them — the schema has no foreign keys — so the
	// uploads would stay in storage forever, paid for and unreachable.
	purgeFiles(repos, files, id)

	if _, err := repos.Nodes.PurgeActorContent(id); err != nil {
		return fmt.Errorf("purging content: %w", err)
	}

	tombstoned, err := repos.Nodes.TombstoneActor(id)
	if err != nil {
		return fmt.Errorf("tombstoning: %w", err)
	}

	if !tombstoned {
		// Restored between the queue read and now. The content purge has
		// already run, which is unfortunate and unavoidable without a lock
		// spanning both — but restoring inside that window is a race measured
		// in milliseconds against a thirty-day deadline.
		log.Printf("sweeper: %d was not tombstoned, likely restored mid-purge", id)
		return nil
	}

	// Audit before the event. The sweeper is the one actor in this flow with no
	// person behind it, so deleted_by is the account itself — the row records
	// that the deadline did this, not that somebody chose to.
	if err := repos.NodeDeletionLogs.Add(
		id, node.Alias, node.Type, id, repositories.DeletionActionPurged,
	); err != nil {
		log.Printf("sweeper: audit write for %d failed: %v", id, err)
	}

	log.Printf("sweeper: purged account %d", id)

	// Tells auth-service to remove the credentials, auth logs and pending
	// tokens that belong to it. Published after the work, so it describes what
	// has happened rather than what was intended.
	//
	// Best-effort: a failure here leaves credentials for an account with no
	// content, which is untidy but not dangerous — the account is disabled and
	// every session was revoked when it was first deleted.
	if publisher != nil {
		// Built the way every other publisher in this service builds one — see
		// actorHandler.Edit and Add. Service and Type are NOT usable raw: Type
		// is "node.actor.group-service.group.v1" and Service is
		// "group-service", so passing them straight in produced
		// "node.actor.group-service.node.actor.group-service.group.v1.purged.v1"
		// — eleven segments against a binding of node.actor.*.*.*.v1, where
		// AMQP's * matches exactly one word. It matched nothing, so
		// auth-service never heard about a purge and the credentials, auth
		// logs and pending tokens it owns were never removed.
		routingKey := purgedRoutingKey(node)
		payload := &event.EventPayload{
			Name:      routingKey,
			ObjectID:  id,
			TargetID:  id,
			SubjectID: id,
		}

		if err := publisher.Publish(payload, routingKey); err != nil {
			log.Printf("sweeper: publishing purge of %d failed: %v", id, err)
		}
	}

	return nil
}

// purgeFiles deletes the uploaded objects belonging to an actor and everything
// it owns.
//
// Best-effort and never fatal. A leaked object costs storage; a sweep that
// aborts here would leave the account's content in the database past its
// deadline, which is the worse failure — and one the person was promised would
// not happen.
//
// No-op when the file service is not configured, which is how the gRPC server
// runs. The sweeper job sets FILE_SERVICE_ADDR; a deployment that has not is
// no worse off than before this existed.
func purgeFiles(repos *repositories.Repositories, files services.FilesGRPCInterface, ownerID int64) {
	if files == nil {
		return
	}

	nodes, err := repos.Nodes.BrowseActorFiles(ownerID)
	if err != nil {
		log.Printf("sweeper: listing files for %d failed: %v", ownerID, err)
		return
	}

	deleted := 0
	for _, node := range nodes {
		for _, path := range filePaths(node) {
			req := &services.FileDeleteRequest{Path: path}
			if err := files.Delete(req); err != nil {
				// Already gone, or storage is unavailable. Either way the next
				// statement destroys the row, so there is no second chance —
				// logged loudly rather than retried.
				log.Printf("sweeper: deleting %s failed: %v", path, err)
				continue
			}
			deleted++
		}
	}

	if deleted > 0 {
		log.Printf("sweeper: deleted %d file(s) for account %d", deleted, ownerID)
	}
}

// filePaths collects every stored object for one node.
//
// Each helper returns the size variants for its kind — an avatar is several
// objects, not one — and each returns an error for a node that has none of that
// kind, which is the ordinary case and not worth logging.
func filePaths(node *entities.AnahitaNode) []string {
	var paths []string

	if avatars, err := node.GetAvatars(); err == nil {
		for _, image := range avatars {
			paths = append(paths, image.Url)
		}
	}

	if covers, err := node.GetCovers(); err == nil {
		for _, image := range covers {
			paths = append(paths, image.Url)
		}
	}

	if portraits, err := node.GetPortraits(); err == nil {
		for _, image := range portraits {
			paths = append(paths, image.Url)
		}
	}

	return paths
}

// purgedRoutingKey builds the routing key for a purge.
//
// Defensive about the split that every other publisher does bare: a node whose
// Type is not the expected five-segment form would panic on index [3] and take
// the whole sweep down with it, mid-purge, for one malformed row.
func purgedRoutingKey(node *entities.AnahitaNode) string {
	entity := node.Type
	if parts := strings.Split(node.Type, "."); len(parts) > 3 {
		entity = parts[3]
	}

	service := node.Service
	if parts := strings.Split(node.Service, "-"); len(parts) > 0 {
		service = parts[0]
	}

	return fmt.Sprintf(event.EventFormatNodeActorPurged, service, entity)
}
