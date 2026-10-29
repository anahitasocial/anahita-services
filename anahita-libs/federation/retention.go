package federation

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// Keeping the posts and the people kept from other servers within bounds.
//
// Three rules, each a setting of the installation's. The first is on as
// it comes; the other two are off, because what they remove is not got
// back by asking:
//
//   - the posts of somebody nobody here follows any more go, once they
//     have been left alone for REMOTE_UNFOLLOWED_DAYS;
//   - with REMOTE_POSTS_DAYS, any post from elsewhere goes once it has
//     been left alone that long;
//   - with REMOTE_POSTS_KEPT, no more than that many posts are kept of
//     one person elsewhere, the newest.
//
// Whatever the rule, what somebody here has a stake in stays: a post they
// liked or saved, a post that anything here or elsewhere stands under or
// on (an answer, a repost, a quote), a post an open report is about. Only
// a post that begins a thread is ever removed by a rule: an answer is a
// few words in somebody's conversation. And nothing written here is
// looked at.
//
// A person elsewhere goes when nothing is left of them and nothing here
// points at them: no post, no follow either way, no like, no block, no
// open report.

// UnfollowedDays is how long the posts of somebody nobody here follows are
// kept after they were last touched: REMOTE_UNFOLLOWED_DAYS, thirty unless
// said, 0 for good.
func UnfollowedDays() int { return daysFrom("REMOTE_UNFOLLOWED_DAYS", 30) }

// PostsDays is how long any post from elsewhere is kept after it was last
// touched: REMOTE_POSTS_DAYS, 0 (for good) unless said.
func PostsDays() int { return daysFrom("REMOTE_POSTS_DAYS", 0) }

// PostsKept is the most posts kept of one person elsewhere:
// REMOTE_POSTS_KEPT, 0 (no limit) unless said.
func PostsKept() int { return daysFrom("REMOTE_POSTS_KEPT", 0) }

func daysFrom(name string, unless int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return unless
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return unless
	}
	return value
}

// A post from one kind of server that begins a thread and that nothing
// here depends on.
const removablePost = `
SELECT post.id
FROM nodes AS post
JOIN nodes AS owner ON owner.id = post.owner_id
WHERE post.type IN (?, ?)
  AND post.uri IS NOT NULL
  AND post.parent_id IS NULL
  AND owner.type = ?
  AND NOT EXISTS (
      SELECT 1
      FROM edges AS held
      JOIN nodes AS who ON who.id = held.node_a_id
      WHERE held.node_b_id = post.id
        AND held.type IN (?, ?)
        AND who.uri IS NULL
  )
  AND NOT EXISTS (
      SELECT 1
      FROM nodes AS under
      WHERE under.parent_id = post.id
         OR under.root_id = post.id
         OR under.quote_id = post.id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM abuse_cases AS reported
      WHERE reported.target_id = post.id
        AND reported.status = 'open'
  )
  AND (`

// Nobody here follows whoever wrote it, and it has been left alone.
const ruleUnfollowed = `(
      post.updated_at < ?
      AND NOT EXISTS (
          SELECT 1
          FROM edges AS follow
          JOIN nodes AS follower ON follower.id = follow.node_a_id
          WHERE follow.node_b_id = owner.id
            AND follow.type = ?
            AND follower.uri IS NULL
      )
  )`

// It has been left alone, whoever wrote it.
const ruleOld = `(post.updated_at < ?)`

// It is past the newest so many of whoever wrote it.
const ruleMany = `(
      post.id IN (
          SELECT ranked.id
          FROM (
              SELECT counted.id,
                     ROW_NUMBER() OVER (PARTITION BY counted.owner_id ORDER BY counted.created_at DESC, counted.id DESC) AS place
              FROM nodes AS counted
              WHERE counted.uri IS NOT NULL
                AND counted.parent_id IS NULL
                AND counted.type IN (?, ?)
          ) AS ranked
          WHERE ranked.place > ?
      )
  )`

// Retention is the rules as they are set.
type Retention struct {
	UnfollowedDays int
	PostsDays      int
	PostsKept      int
}

// RetentionFromEnv reads the rules the installation set.
func RetentionFromEnv() Retention {
	return Retention{UnfollowedDays: UnfollowedDays(), PostsDays: PostsDays(), PostsKept: PostsKept()}
}

// OldPosts finds the posts from one kind of server that a rule says are
// to go, the longest left alone first. With no rule set, none.
func OldPosts(db *sql.DB, ownerType string, rules Retention, now time.Time, limit int) ([]int64, error) {
	args := []any{
		constants.NodeMediumNoteEntityType, constants.NodeMediumPhotoEntityType,
		ownerType,
		constants.EdgeBaseLikeEntityType, constants.EdgeBaseBookmarkEntityType,
	}
	var any []string
	if rules.UnfollowedDays > 0 {
		any = append(any, ruleUnfollowed)
		args = append(args, now.AddDate(0, 0, -rules.UnfollowedDays).UTC(), constants.EdgeFollowEntityType)
	}
	if rules.PostsDays > 0 {
		any = append(any, ruleOld)
		args = append(args, now.AddDate(0, 0, -rules.PostsDays).UTC())
	}
	if rules.PostsKept > 0 {
		any = append(any, ruleMany)
		args = append(args, constants.NodeMediumNoteEntityType, constants.NodeMediumPhotoEntityType, rules.PostsKept)
	}
	if len(any) == 0 {
		return nil, nil
	}
	query := removablePost + strings.Join(any, "\n   OR ") + `
  )
ORDER BY post.updated_at, post.id
LIMIT ?`
	args = append(args, limit)

	return idsOf(db, query, args...)
}

// Somebody of one kind of server of whom nothing is left here and at whom
// nothing here points, and who has been left alone.
const stalePeople = `
SELECT person.id
FROM nodes AS person
WHERE person.type = ?
  AND person.uri IS NOT NULL
  AND person.updated_at < ?
  AND NOT EXISTS (
      SELECT 1
      FROM nodes AS theirs
      WHERE theirs.owner_id = person.id
         OR theirs.created_by = person.id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM edges AS tie
      WHERE tie.node_a_id = person.id
         OR tie.node_b_id = person.id
  )
  AND NOT EXISTS (
      SELECT 1
      FROM abuse_cases AS reported
      WHERE reported.target_id = person.id
        AND reported.status = 'open'
  )
ORDER BY person.updated_at, person.id
LIMIT ?`

// StalePeople finds them. With the unfollowed rule off, none: it is the
// same rule that says how long what nobody follows is kept.
func StalePeople(db *sql.DB, ownerType string, rules Retention, now time.Time, limit int) ([]int64, error) {
	if rules.UnfollowedDays <= 0 {
		return nil, nil
	}
	return idsOf(db, stalePeople, ownerType, now.AddDate(0, 0, -rules.UnfollowedDays).UTC(), limit)
}

func idsOf(db *sql.DB, query string, args ...any) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// The photo posts kept of one person from elsewhere: what is stored of
// their pictures is removed with them, when that person is (refused, or
// gone where they were).
const photoPostsOf = `
SELECT id
FROM nodes
WHERE owner_id = ?
  AND type = ?
  AND uri IS NOT NULL`

// PhotoPostsOf finds them.
func PhotoPostsOf(db *sql.DB, ownerID int64) ([]int64, error) {
	return idsOf(db, photoPostsOf, ownerID, constants.NodeMediumPhotoEntityType)
}

// RemovePost takes a post from elsewhere away, with what is stored of its
// pictures. It answers with the post as it was, or nil when there was
// nothing to take.
func (k *Keeper) RemovePost(postID int64) (*entities.AnahitaNode, error) {
	post, err := k.Nodes.Read(postID)
	if err != nil || post == nil || post.ID == 0 || !post.IsRemote() || post.OwnerID <= 0 {
		return nil, err
	}

	var files []*entities.PhotoFile
	if post.Type == constants.NodeMediumPhotoEntityType && k.PhotoFiles != nil {
		files, _ = k.PhotoFiles.BrowsePhotoFiles([]int64{post.ID})
	}
	deleted, err := k.Nodes.DeleteRemote(post.URI, post.OwnerID)
	if err != nil || !deleted {
		return nil, err
	}
	if len(files) > 0 {
		k.DeletePostPictures(post, files)
	}
	return post, nil
}
