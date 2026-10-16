package repositories

import (
	"context"
	"time"
)

// Published dates a draft that has just been published as made now: it is
// a post from this moment, in a feed and elsewhere, not from when it was
// begun (the user, 2026-10-16).
func (r *nodesRepo) Published(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE nodes SET created_at = ?, updated_at = ? WHERE id = ?`,
		now, now, id)
	return err
}
