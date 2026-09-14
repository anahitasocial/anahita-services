package repositories

import (
	"context"
	"errors"

	"anahita-libs/constants"
)

// ErrLastSuperAdmin is returned when a lifecycle transition would leave the
// installation with nobody able to administer it.
//
// A sentinel rather than a string, so callers can answer 409 with a body that
// explains the way out instead of a 500 that does not. It crosses the gRPC
// boundary as its message, which is what person-service matches on — see
// isLastSuperAdmin there.
var ErrLastSuperAdmin = errors.New("last_super_administrator")

// countOtherActiveSuperAdmins counts the super administrators who are not this
// person and can still act.
//
// "Can still act" is doing real work in that sentence. A super administrator
// with deleted_at set is inside their grace period and will be purged, so
// counting them satisfies the floor with somebody who will not be there and
// the site reaches zero one deletion later. A disabled one cannot sign in to
// administer anything. Neither belongs in the count.
//
// A single-table query now that person_type lives on `nodes`. It used to need a
// join to people_people from person-service, which is why the check lived there
// — and why it was duplicated across four handlers, each of which had to
// remember to run it.
func (r *nodesRepo) countOtherActiveSuperAdmins(ctx context.Context, excludeID int64) (int, error) {
	// Narrowed to PERSON nodes, not merely to the person_type value.
	//
	// person_type is a column on `nodes`, and every node carries one — an
	// article, a photo, a hashtag. A stray write can put a value in any of
	// them, and one narrowed on `alias` did exactly that during development,
	// leaving a hashtag counted as a super administrator.
	//
	// Here the failure is the dangerous direction: an inflated count makes the
	// guard believe somebody else can still administer the site, and it then
	// permits the deletion that removes the last real one.
	const query = `
SELECT COUNT(*)
FROM nodes
WHERE type = ?
  AND person_type = ?
  AND id <> ?
  AND deleted_at IS NULL
  AND enabled = 1`

	var count int
	err := r.db.QueryRowContext(ctx, query,
		constants.NodeActorPersonEntityType,
		constants.PersonTypeSuperAdmin,
		excludeID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// guardLastSuperAdmin refuses a transition that would remove the last person
// able to administer the installation.
//
// Returns nil for anything that is not a super administrator, which is almost
// every call — the cost is one indexed lookup on person_type.
//
// Enforced here rather than in the calling service because this is the floor
// under the whole installation, and a floor that each caller has to remember to
// check is one a new caller will not. person-service had the same check written
// out four times, in Delete, Archive, SetEnabled and Purge.
func (r *nodesRepo) guardLastSuperAdmin(ctx context.Context, id int64) error {
	var personType string
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(person_type, '') FROM nodes WHERE id = ?`, id).Scan(&personType)
	if err != nil {
		return err
	}

	if personType != constants.PersonTypeSuperAdmin {
		return nil
	}

	others, err := r.countOtherActiveSuperAdmins(ctx, id)
	if err != nil {
		return err
	}

	if others == 0 {
		return ErrLastSuperAdmin
	}

	return nil
}
