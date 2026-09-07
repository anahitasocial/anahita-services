// Package purge destroys an actor and everything it owned.
//
// Its own package because two callers need exactly one implementation of it:
// the sweeper, when a grace period runs out, and the super-administrator hard
// delete, which skips the waiting. Two copies of a destructive operation that
// drift is the trap the two restore implementations are already in, and this
// one cannot be undone.
//
// The package is named for what it does rather than for either caller, so
// neither reads as borrowing the other's code.
package purge

import (
	"fmt"
	"log"
	"strings"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"

	"graph/data/repositories"
)

// Account destroys one account's content and tombstones it.
//
// The caller decides WHETHER to do this — the sweeper because a deadline
// passed, the hard-delete endpoint because a super administrator said so. This
// only decides how, and it notifies nobody: the purged event it publishes is
// for auth-service to clean up credentials, not for anybody to be told.
//
// Ordering matters. The content goes first and the tombstone second, so an
// interruption between them leaves an account still marked deleted and still in
// the queue — it gets picked up next run and finishes. The reverse order would
// produce a tombstone with a live content graph hanging off it and nothing left
// to find that content by.
func Account(repos *repositories.Repositories, publisher *event.Publisher, files services.FilesGRPCInterface, id int64) error {
	// Read before destroying: the event needs the type, and once the content is
	// gone this is the only chance to learn anything about what was purged.
	node, err := repos.Nodes.Read(id)
	if err != nil {
		return fmt.Errorf("reading node: %w", err)
	}

	// Mark it deleted first, if it is not already.
	//
	// TombstoneActor is guarded on `deleted_at IS NOT NULL`, which the sweeper
	// always satisfies — it only ever purges things that have been scheduled.
	// The hard delete does not: a super administrator destroying a LIVE
	// account left that guard unmet, so the content was purged and the
	// tombstone silently matched no rows. The account kept its name, its body
	// and its handle while everything it had written was gone.
	//
	// SoftDeleteActor is idempotent — guarded on `deleted_at IS NULL` — so
	// this is a no-op on the sweeper's path and the missing step on the
	// other.
	//
	// It also improves the failure mode. Anything that goes wrong from here
	// leaves the account in the ordinary deleted state, which the sweeper
	// already knows how to finish, rather than in a state nothing recognises.
	if _, err := repos.Nodes.SoftDeleteActor(id); err != nil {
		return fmt.Errorf("marking deleted: %w", err)
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
