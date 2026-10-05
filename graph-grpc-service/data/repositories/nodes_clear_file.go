package repositories

import (
	"context"
	"errors"
)

// The images a node can be without. Which column each is kept in.
const (
	NodeFileAvatar = "avatar"
	NodeFileCover  = "cover"
)

// ErrUnknownNodeFile is returned for a kind of image that is not one of the
// above.
var ErrUnknownNodeFile = errors.New("unknown_node_file")

// An actor's avatar is kept in `filename`, which for a photo post is the
// photo itself. See docs/architecture.md, Naming images.
const clearNodeAvatar = `
UPDATE nodes
SET filename = '',
    modified_by = ?,
    updated_at = UTC_TIMESTAMP()
WHERE id = ?
  AND type LIKE 'node.actor.%'`

const clearNodeCover = `
UPDATE nodes
SET cover_filename = '',
    modified_by = ?,
    updated_at = UTC_TIMESTAMP()
WHERE id = ?`

// ClearFile takes an avatar or a cover off a node.
//
// Its own write because Edit cannot do it. Edit sets a filename only when it
// is given one, so that an edit which does not mention the image leaves it
// alone; the price was that handing it an empty name changed nothing.
// Removing an avatar therefore deleted the stored image and left the profile
// naming it, and the profile showed a broken picture from then on.
//
// The avatar is cleared only on an actor. The same column on a photo post
// names the photo, and clearing that is not what anybody asking to remove an
// avatar means.
//
// Returns false when nothing changed: no such node, or not an actor.
func (r *nodesRepo) ClearFile(id int64, kind string, modifiedBy int64) (bool, error) {
	var query string
	switch kind {
	case NodeFileAvatar:
		query = clearNodeAvatar
	case NodeFileCover:
		query = clearNodeCover
	default:
		return false, ErrUnknownNodeFile
	}

	if id <= 0 {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	result, err := r.db.ExecContext(ctx, query, modifiedBy, id)
	if err != nil {
		return false, err
	}

	changed, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return changed > 0, nil
}
