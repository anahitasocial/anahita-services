package repositories

import (
	"context"
)

// TouchLastVisit stamps nodes.last_visit_date with the current time.
//
// This column existed on the old profile table and was written ONLY by the
// INSERT that created the row — never on a sign-in — so for all 822 people it
// recorded the date they joined under a name that says otherwise. Anything
// reading it to find dormant accounts was reading the join date.
//
// It is deliberately narrow. Routing a login through Edit would write the
// whole actor from whatever the caller last read, and publish an "actor
// edited" event — republishing every signing-in actor to search and the feeds,
// and letting a stale read overwrite a profile as a side effect of a sign-in.
//
// No last-super-admin guard and no viewer. This cannot reduce anybody's
// access, and the person doing it has already authenticated as themselves.
//
// Refuses a soft-deleted actor. A deleted account can still reach the sign-in
// form — that is how restore-on-login works — and a visit stamped before the
// restore is decided would say somebody used an account that, at that moment,
// did not exist. The restore path stamps it once the account is actually back.
//
// Returns false when nothing was stamped: no such actor, or deleted. The
// caller needs to tell that from success, even though it never blocks a login
// on it.
func (r *nodesRepo) TouchLastVisit(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if id <= 0 {
		return false, nil
	}

	// NOW() rather than a timestamp from the caller: the value means "when the
	// database saw this", and taking it from a client makes it a claim from a
	// machine whose clock nobody controls.
	const query = `
UPDATE nodes
SET last_visit_date = NOW()
WHERE id = ? AND deleted_at IS NULL;
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
