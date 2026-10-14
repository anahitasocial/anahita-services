package federation

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// Keeping the pictures of posts from other servers within bounds.
//
// A picture is nearly all of what a post from elsewhere costs to keep: it
// is stored at every size a photo is, and nothing ever removed it. So the
// pictures of a post that came from elsewhere are removed from storage
// once the post has been left alone for a while. The post stays, with its
// words and what each picture showed, and says that its pictures are not
// kept here any more. Whoever wants to see them asks, and they are copied
// from where they came from again, as they were the first time.
//
// It is how Mastodon and GoToSocial keep what they hold of other servers
// from growing for ever, and it is on unless an installation says
// otherwise, since it loses nothing that is still there to be had.
//
// Never the pictures of a post somebody here liked, saved, answered,
// reposted or quoted, nor of one an open report is about. And never
// anything written here: only what has an address on another server is
// looked at.

// MediaDays is how many days the pictures of a post from elsewhere are
// kept after the post was last touched: REMOTE_MEDIA_DAYS, thirty unless
// said, and 0 for keeping them for good.
func MediaDays() int {
	raw := os.Getenv("REMOTE_MEDIA_DAYS")
	if raw == "" {
		return 30
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 0 {
		return 30
	}
	return days
}

// Where a post says, beside its words, that its pictures were removed.
const mediaMetaKey = "media_removed"

// RemovedMedia is what a post keeps of pictures that are no longer stored.
type RemovedMedia struct {
	At time.Time `json:"at"`
	// What each picture showed, as its author said: one for each picture,
	// "" where nothing was said. How many there were is how many of these.
	Alts []string `json:"alts"`
	// They were asked for again and are not where they came from any
	// more: the post was taken away there, or its server is gone.
	Gone bool `json:"gone,omitempty"`
}

// MediaRemovedFrom reads what a post says of pictures that were removed,
// or nil when it says nothing.
func MediaRemovedFrom(meta []byte) *RemovedMedia {
	if len(meta) == 0 {
		return nil
	}
	fields := map[string]json.RawMessage{}
	if json.Unmarshal(meta, &fields) != nil {
		return nil
	}
	raw, marked := fields[mediaMetaKey]
	if !marked {
		return nil
	}
	removed := &RemovedMedia{}
	if json.Unmarshal(raw, removed) != nil {
		return nil
	}
	return removed
}

// WithMediaRemoved says in a post's meta that its pictures were removed,
// or with nil that they are there again. What else the meta holds is left.
func WithMediaRemoved(meta []byte, removed *RemovedMedia) []byte {
	fields := map[string]json.RawMessage{}
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &fields)
	}
	if removed == nil {
		delete(fields, mediaMetaKey)
	} else {
		raw, err := json.Marshal(removed)
		if err != nil {
			return meta
		}
		fields[mediaMetaKey] = raw
	}
	if len(fields) == 0 {
		return nil
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return meta
	}
	return out
}

// The photo posts from one kind of server whose pictures are due to go:
// left alone since before a time, with pictures still stored, and with
// nothing here that depends on them.
//
// "Left alone" is by when the post was last written here, which is when
// it arrived, when its author last changed it, and when its pictures were
// last copied again: so pictures that were asked for are kept as long
// again.
const oldPicturePosts = `
SELECT post.id
FROM nodes AS post
JOIN nodes AS owner ON owner.id = post.owner_id
WHERE post.type = ?
  AND post.uri IS NOT NULL
  AND owner.type = ?
  AND post.updated_at < ?
  AND EXISTS (
      SELECT 1
      FROM photo_files AS file
      WHERE file.node_id = post.id
  )
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
      FROM nodes AS made
      WHERE made.uri IS NULL
        AND (made.parent_id = post.id OR made.root_id = post.id OR made.quote_id = post.id)
  )
  AND NOT EXISTS (
      SELECT 1
      FROM abuse_cases AS reported
      WHERE reported.target_id = post.id
        AND reported.status = 'open'
  )
ORDER BY post.updated_at
LIMIT ?`

// OldPicturePosts finds them: `ownerType` is the kind of person the posts
// are of, so that each service looks after what came through it.
func OldPicturePosts(db *sql.DB, ownerType string, before time.Time, limit int) ([]int64, error) {
	return idsOf(db, oldPicturePosts,
		constants.NodeMediumPhotoEntityType, ownerType, before.UTC(),
		constants.EdgeBaseLikeEntityType, constants.EdgeBaseBookmarkEntityType,
		limit)
}

// RemovePictures takes a post's pictures out of storage and has the post
// say so. It answers with whether there were any.
//
// The post is told first and the files go last: stopped half way, it says
// its pictures are gone while some are still stored, which costs a little
// room, and never shows a picture that is not there.
func (k *Keeper) RemovePictures(postID int64, now time.Time) (bool, error) {
	post, err := k.Nodes.Read(postID)
	if err != nil || post == nil || post.ID == 0 || !post.IsRemote() || post.Type != constants.NodeMediumPhotoEntityType {
		return false, err
	}
	files, err := k.PhotoFiles.BrowsePhotoFiles([]int64{post.ID})
	if err != nil || len(files) == 0 {
		return false, err
	}

	alts := make([]string, 0, len(files))
	for _, file := range files {
		alts = append(alts, file.AltText)
	}
	if err := k.remark(post, &RemovedMedia{At: now.UTC(), Alts: alts}); err != nil {
		return false, err
	}
	if _, err := k.PhotoFiles.SetPhotoFiles(post.ID, nil, nil); err != nil {
		return false, err
	}
	k.DeletePostPictures(post, files)
	return true, nil
}

// remark writes what a post says of its removed pictures, leaving the
// rest of it as it is.
func (k *Keeper) remark(post *entities.AnahitaNode, removed *RemovedMedia) error {
	update := entities.NewAnahitaNode()
	update.Type, update.URI, update.OriginDomain = post.Type, post.URI, post.OriginDomain
	update.OwnerID, update.ParentID = post.OwnerID, post.ParentID
	update.Name, update.Body, update.Language, update.WebsiteURL = post.Name, post.Body, post.Language, post.WebsiteURL
	update.Meta = WithMediaRemoved(post.Meta, removed)

	_, _, err := k.Nodes.UpsertRemote(update)
	return err
}

// PicturesGone has a post say that its pictures were asked for again and
// are no longer where they came from.
func (k *Keeper) PicturesGone(post *entities.AnahitaNode) error {
	removed := MediaRemovedFrom(post.Meta)
	if removed == nil || removed.Gone {
		return nil
	}
	removed.Gone = true
	return k.remark(post, removed)
}

// RestorePictures copies a post's pictures here again and has the post
// stop saying they were removed. It answers with how many are stored now.
// With none, the post is left saying what it said.
func (k *Keeper) RestorePictures(post *entities.AnahitaNode, pictures []Picture) (int, error) {
	if post == nil || MediaRemovedFrom(post.Meta) == nil {
		return 0, nil
	}
	k.CopyPostPictures(post, pictures)

	files, err := k.PhotoFiles.BrowsePhotoFiles([]int64{post.ID})
	if err != nil || len(files) == 0 {
		return 0, err
	}
	return len(files), k.remark(post, nil)
}

// RemoveOldPictures removes the pictures of the posts that are due, of one
// kind of server, and answers with how many posts that was. With no days
// set, nothing is removed.
func (k *Keeper) RemoveOldPictures(db *sql.DB, ownerType string, now time.Time, limit int) int {
	days := MediaDays()
	if days <= 0 || db == nil || k.Nodes == nil || k.PhotoFiles == nil || k.Files == nil {
		return 0
	}

	ids, err := OldPicturePosts(db, ownerType, now.AddDate(0, 0, -days), limit)
	if err != nil {
		log.Println("the posts whose pictures are due to go could not be read:", err)
		return 0
	}
	removed := 0
	for _, id := range ids {
		had, err := k.RemovePictures(id, now)
		if err != nil {
			log.Printf("the pictures of post %d were not removed: %v", id, err)
			continue
		}
		if had {
			removed++
		}
	}
	return removed
}
