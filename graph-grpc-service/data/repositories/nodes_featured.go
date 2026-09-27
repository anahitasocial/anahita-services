package repositories

import (
	"context"
	"errors"
)

// ErrNotFeaturable is returned when featuring an actor that is disabled,
// archived or deleted.
//
// The featured list is what onboarding offers a new member to follow, and
// every one of those states is a reason not to: a disabled account is
// suspended, an archived one is finished and no longer pushed at anybody, and
// a deleted one is on its way out. The list's own query drops them too — this
// refuses the write so that a super administrator is told, rather than
// featuring somebody who then silently never appears.
//
// The message is what crosses gRPC, and callers match on it.
var ErrNotFeaturable = errors.New("not_featurable")

// SetFeatured features or unfeatures an actor.
//
// A statement by the installation, not by the actor, so it is its own
// single-purpose write rather than a field on Edit — which writes the whole
// node from whatever it was handed.
//
// Featuring stamps featured_at only when it is not already set, so featuring
// twice does not move the actor to the top of the list; the date is also the
// order, and re-featuring is not a way to bump somebody. Unfeaturing clears it
// whatever state the actor is in — taking a disabled account off the list is
// always allowed.
//
// Returns false when nothing changed: already in that state, or absent.
func (r *nodesRepo) SetFeatured(id int64, featured bool) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if id <= 0 {
		return false, nil
	}

	if !featured {
		const query = `
UPDATE nodes
SET featured_at = NULL
WHERE id = ? AND featured_at IS NOT NULL;
`
		return r.execChanged(ctx, query, id)
	}

	// The lifecycle guard is in the statement, not only in the check below,
	// so an actor disabled between the two cannot slip onto the list.
	const query = `
UPDATE nodes
SET featured_at = NOW()
WHERE id = ? AND featured_at IS NULL
  AND enabled = 1 AND archived_at IS NULL AND deleted_at IS NULL;
`
	changed, err := r.execChanged(ctx, query, id)
	if err != nil || changed {
		return changed, err
	}

	// Nothing changed. Either it was already featured, which is success, or it
	// is in a state that cannot be featured, which the caller has to be told.
	const check = `
SELECT COUNT(*) FROM nodes
WHERE id = ? AND (enabled = 0 OR archived_at IS NOT NULL OR deleted_at IS NOT NULL);
`
	var refused int
	if err := r.db.QueryRowContext(ctx, check, id).Scan(&refused); err != nil {
		return false, err
	}
	if refused > 0 {
		return false, ErrNotFeaturable
	}

	return false, nil
}

// execChanged runs a single-row UPDATE and reports whether it touched a row.
func (r *nodesRepo) execChanged(ctx context.Context, query string, args ...any) (bool, error) {
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}
