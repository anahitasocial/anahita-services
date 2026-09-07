package repositories

import (
	"context"

	"anahita-libs/data/entities"
)

// The destructive half of deletion, run by the sweeper once the grace period
// has expired.
//
// Two steps rather than one, and the split is the point:
//
//	PurgeActorContent — removes everything the actor owned
//	TombstoneActor    — strips the actor node but KEEPS it
//
// PurgeByOwner did both at once and deleted the actor row along with
// everything else. That row has to survive, for two reasons that turn out to be
// the same reason:
//
//   - ActivityPub identifies an actor by a permanent URI and remote servers
//     cache actor objects against it. A handle that returns to the pool is a
//     handle whose next owner inherits the previous person's followers,
//     mentions and cached profile on every instance that ever saw them. A
//     surviving row reserves the handle and is what lets the actor URI answer
//     410 Gone so remote servers drop their cache. Mastodon keeps a tombstone
//     for exactly this.
//   - Every other table in this schema references people by bare id with no
//     foreign key. Deleting the row leaves `created_by`, `modified_by` and
//     `last_comment_by` across the database pointing at nothing.
//
// AT Protocol is relaxed about handle reuse — identity there is a permanent DID
// and the handle only points at it — so reserving costs nothing under it and is
// close to mandatory under ActivityPub.

// PurgeActorContent deletes every node the actor owns, every descendant of
// those by parent_id, and every edge touching any of them — but not the actor
// node itself.
//
// This is PurgeByOwner's first two statements. The third, which deleted the
// owner, is deliberately absent: TombstoneActor takes its place.
//
// NOTE ON THE RECURSION. The descendant walk takes other people's replies with
// it — a comment somebody left on this actor's post is a child of that post.
// That is the ordinary meaning of deleting a post, and every platform does it;
// what matters is that the person is told before they confirm, which belongs in
// the consequence screen rather than here.
func (r *nodesRepo) PurgeActorContent(ownerID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if ownerID <= 0 {
		return false, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Edges first. Once the nodes are gone there is nothing left to find them
	// by, and this schema has no foreign keys to clean up after us.
	const deleteEdges = `
WITH RECURSIVE content (id) AS (
  SELECT id
  FROM nodes
  WHERE owner_id = ?

  UNION ALL

  SELECT c.id
  FROM nodes c
  JOIN content p ON c.parent_id = p.id
),
targets (id) AS (
  SELECT id FROM content
  UNION ALL
  SELECT ?  -- the actor itself: its follows, admin edges and votes go too
)
DELETE e
FROM edges e
JOIN targets t
  ON e.node_a_id = t.id
  OR e.node_b_id = t.id;
`

	if _, err = tx.ExecContext(ctx, deleteEdges, ownerID, ownerID); err != nil {
		return false, err
	}

	const deleteContentNodes = `
WITH RECURSIVE content (id) AS (
  SELECT id
  FROM nodes
  WHERE owner_id = ?

  UNION ALL

  SELECT c.id
  FROM nodes c
  JOIN content p ON c.parent_id = p.id
)
DELETE n
FROM nodes n
JOIN content c ON n.id = c.id;
`
	if _, err = tx.ExecContext(ctx, deleteContentNodes, ownerID); err != nil {
		return false, err
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}

// TombstoneActor strips an actor node down to an identity marker.
//
// What survives: the id, the alias, the type, and deleted_at. What goes: the
// name, body, excerpt, status, avatar and cover filenames, geo fields, meta,
// website_url,
// and the counters. Everything that made it somebody's profile.
//
// The alias is kept ON PURPOSE — it is the whole reservation. Scrubbing it
// would release the handle and reintroduce the impersonation this exists to
// prevent.
//
// Stays disabled, so every read path that filters on `enabled` keeps skipping
// it and the tombstone never surfaces as a profile.
//
// The one case this does not cover is a verified erasure request that reaches
// the handle itself. That wants a separate, deliberate admin action rather than
// being the default, because it trades the reservation away.
func (r *nodesRepo) TombstoneActor(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if id <= 0 {
		return false, nil
	}

	const query = `
UPDATE nodes
SET
  name = '',
  body = NULL,
  excerpt = NULL,
  status = NULL,
  status_update_time = NULL,
  meta = NULL,
  website_url = NULL,
  filename = NULL,
  filesize = NULL,
  cover_filename = NULL,
  cover_filesize = NULL,
  cover_mimetype = NULL,
  geo_latitude = NULL,
  geo_longitude = NULL,
  geo_address = NULL,
  geo_city = NULL,
  geo_state_province = NULL,
  geo_country = NULL,
  geo_postalcode = NULL,
  actor_gender = NULL,
  permissions = NULL,
  enabled_optional_features = NULL,
  comment_count = 0,
  repost_count = 0,
  quote_count = 0,
  follower_count = 0,
  leader_count = 0,
  mutual_count = 0,
  subscriber_count = 0,
  vote_up_count = 0,
  vote_down_count = 0,
  hits = 0,
  last_comment_id = NULL,
  last_comment_by = NULL,
  last_comment_on = NULL,
  enabled = 0
WHERE id = ? AND deleted_at IS NOT NULL;
`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// BrowseActorFiles returns the actor node and every node it owns that carries
// an uploaded file.
//
// Called BEFORE the purge, because once the rows are gone there is nothing left
// to say which objects belonged to them — and the schema has no foreign keys,
// so nothing else knows either. Uploaded avatars, covers and photos would
// otherwise stay in object storage forever, paid for and unreachable.
//
// Only the columns the image helpers read are selected: GetAvatars, GetCovers
// and GetPortraits build their paths from id, type, filename and
// cover_filename. Selecting whole nodes would pull mediumtext bodies into
// memory for no reason.
func (r *nodesRepo) BrowseActorFiles(ownerID int64) ([]*entities.AnahitaNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if ownerID <= 0 {
		return nil, nil
	}

	// The same recursion PurgeActorContent uses, plus the actor itself, so the
	// set of files matches exactly the set of rows about to be destroyed.
	const query = `
WITH RECURSIVE content (id) AS (
  SELECT id FROM nodes WHERE owner_id = ?
  UNION ALL
  SELECT c.id FROM nodes c JOIN content p ON c.parent_id = p.id
),
targets (id) AS (
  SELECT id FROM content
  UNION ALL
  SELECT ?
)
SELECT n.id, n.type,
       COALESCE(n.filename, ''), COALESCE(n.cover_filename, '')
FROM nodes n
JOIN targets t ON n.id = t.id
WHERE COALESCE(n.filename, '') <> '' OR COALESCE(n.cover_filename, '') <> '';
`

	rows, err := r.db.QueryContext(ctx, query, ownerID, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*entities.AnahitaNode
	for rows.Next() {
		item := entities.NewAnahitaNode()
		if err := rows.Scan(&item.ID, &item.Type, &item.Filename, &item.CoverFilename); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
