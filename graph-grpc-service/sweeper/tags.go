package sweeper

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"graph/data/repositories"
)

// TagGrace is how old an unused hashtag or location must be before it is
// removed: anything newer may belong to a post being written.
const TagGrace = 24 * time.Hour

// Tags removes the hashtags and locations no post carries any more, older
// than TagGrace, but for a location a custom feed names
// (docs/plans/tag-sweep.md). What a purge leaves most of: the people and
// groups go, and the tags only they used stay.
func Tags(repos *repositories.Repositories) repositories.TagSweep {
	swept, err := repos.Tags.SweepUnused(time.Now().UTC().Add(-TagGrace))
	if err != nil {
		log.Printf("sweeper: tags: %v", err)
	}
	if swept.Hashtags > 0 || swept.Locations > 0 {
		log.Printf("sweeper: %d unused hashtag(s) and %d unused location(s) removed", swept.Hashtags, swept.Locations)
	}
	return swept
}

// RunTags is Tags alone, under the sweeper's lock: `sweeper -tags-only`, to
// see a sweep's counts at once after a purge.
func RunTags(db *sql.DB, repos *repositories.Repositories) (repositories.TagSweep, error) {
	acquired, release, err := acquireLock(db, requestLockTimeout)
	if err != nil {
		return repositories.TagSweep{}, fmt.Errorf("acquiring the sweeper lock: %w", err)
	}
	if !acquired {
		return repositories.TagSweep{}, fmt.Errorf("another run held the lock for %d seconds", requestLockTimeout)
	}
	defer release()

	return Tags(repos), nil
}
