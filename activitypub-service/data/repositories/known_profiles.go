package repositories

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"anahita-libs/constants"
)

// KnownProfile is somebody from another network who is kept here, because
// somebody here followed them or they were met in a thread.
type KnownProfile struct {
	// The profile kept here.
	ID int64 `json:"id"`
	// What kind of profile it is, as any node says.
	Type string `json:"type"`
	// `activitypub` or `atproto`.
	Network string `json:"network"`
	Name    string `json:"name"`
	// Their handle, as it is kept here.
	Handle string `json:"handle"`
	// Where they are: their actor's address, or `at://` and their DID.
	URI string `json:"uri"`
	// How many here follow them.
	FollowerCount int64 `json:"follower_count"`
	// When they were first kept.
	CreatedAt time.Time `json:"created_at"`
}

// KnownProfilesInterface is who from other networks is kept here.
type KnownProfilesInterface interface {
	// Browse is a page of them, newest first, and how many there are.
	// `search` narrows it to the names and handles it is in.
	Browse(search string, limit, offset int) ([]*KnownProfile, int, error)
}

type KnownProfiles struct {
	db *sql.DB
}

func NewKnownProfiles(db *sql.DB) *KnownProfiles {
	return &KnownProfiles{db: db}
}

const countKnownProfiles = `
SELECT COUNT(*)
FROM nodes
WHERE type IN (?, ?)
  AND purged_at IS NULL
  AND deleted_at IS NULL
  AND (name LIKE ? ESCAPE '!' OR alias LIKE ? ESCAPE '!')`

const browseKnownProfiles = `
SELECT id, type, name, alias, COALESCE(uri, ''), follower_count, created_at
FROM nodes
WHERE type IN (?, ?)
  AND purged_at IS NULL
  AND deleted_at IS NULL
  AND (name LIKE ? ESCAPE '!' OR alias LIKE ? ESCAPE '!')
ORDER BY created_at DESC, id DESC
LIMIT ? OFFSET ?`

var likeEscaper = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")

func (repo *KnownProfiles) Browse(search string, limit, offset int) ([]*KnownProfile, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	fediverse := constants.NodeActorActivityPubPersonEntityType
	bluesky := constants.NodeActorATProtoPersonEntityType
	pattern := "%" + likeEscaper.Replace(strings.TrimSpace(search)) + "%"

	var total int
	err := repo.db.QueryRowContext(ctx, countKnownProfiles, fediverse, bluesky, pattern, pattern).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := repo.db.QueryContext(ctx, browseKnownProfiles, fediverse, bluesky, pattern, pattern, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	found := []*KnownProfile{}
	for rows.Next() {
		profile := &KnownProfile{Network: "activitypub"}
		if err := rows.Scan(&profile.ID, &profile.Type, &profile.Name, &profile.Handle, &profile.URI,
			&profile.FollowerCount, &profile.CreatedAt); err != nil {
			return nil, 0, err
		}
		if profile.Type == bluesky {
			profile.Network = "atproto"
		}
		found = append(found, profile)
	}
	return found, total, rows.Err()
}
