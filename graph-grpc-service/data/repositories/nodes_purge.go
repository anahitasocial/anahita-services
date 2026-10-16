package repositories

import (
	"context"
	"time"
)

// The destructive half of deletion, run by the sweeper once the grace period
// has expired.
//
// Two steps rather than one, and the split is the point:
//
//	the purge steps  — remove everything the actor owned or wrote
//	TombstoneActor   — strips the actor node but KEEPS it
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
//     foreign key. Deleting the row leaves `created_by` and `modified_by`
//     across the database pointing at nothing.
//
// AT Protocol is relaxed about handle reuse — identity there is a permanent DID
// and the handle only points at it — so reserving costs nothing under it and is
// close to mandatory under ActivityPub.

// TombstoneActor strips an actor node down to an identity marker.
//
// It named three columns that do not exist on `nodes` — actor_gender,
// permissions and enabled_optional_features — so the UPDATE failed with
// "Unknown column" every time. That was invisible until something actually
// purged an account: PurgeActorContent runs FIRST and succeeds, so the failure
// left content destroyed, no tombstone written, and the row still flagged
// deleted — back in the sweeper's queue to fail again on the next run, forever.
//
// Nothing had ever purged anything, so nothing had ever run it. Gender lives on
// people_people, not here; the other two appear to be from a schema this was
// ported from.
//
// What survives: the id, the alias, the type, deleted_at and purged_at, which
// is what takes the actor out of the sweeper's queue. What goes: the
// name, body, excerpt, avatar and cover filenames, geo fields, meta,
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
  meta = NULL,
  website_url = NULL,
  filename = NULL,
  avatar_filename = NULL,
  cover_filename = NULL,
  geo_latitude = NULL,
  geo_longitude = NULL,
  geo_address = NULL,
  geo_city = NULL,
  geo_state_province = NULL,
  geo_country = NULL,
  geo_postalcode = NULL,
  comment_count = 0,
  repost_count = 0,
  quote_count = 0,
  follower_count = 0,
  leader_count = 0,
  subscriber_count = 0,
  vote_up_count = 0,
  view_count = 0,
  enabled = 0,
  purged_at = ?
WHERE id = ? AND deleted_at IS NOT NULL;
`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), id)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}
