package sweeper

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"anahita-libs/data/services"
	"anahita-libs/videos"
)

// The file sweep (anahita-web roadmap row 106, docs/plans/done/file-sweep.md):
// stored files that no row names any more, found and removed once a night.
// A purge whose deletions failed, a replaced avatar's old sizes, a worker
// stopped half-way: each leaves files nobody can reach, and pays for them.
//
// A file goes only when the sweep is sure: its key is in a shape this knows,
// the row it names is certainly not there, and it is a day old. A key in no
// known shape, a folder this does not judge, a query that failed: kept.

// FileStore is what the sweep needs of file-grpc-service: to list the bucket
// a page at a time, and to remove a file. services.FilesGRPC is one.
type FileStore interface {
	Browse(prefix, after string, limit int) ([]services.BrowsedFile, bool, error)
	Delete(req *services.FileDeleteRequest) error
}

const (
	assetsPrefix = "assets/v1/"

	// FileMinAge is how old a file must be, by the store's own date, before
	// the sweep judges it: nothing being uploaded, resized or encoded is
	// touched.
	FileMinAge = 24 * time.Hour

	// FilesPerNight is the most one run removes. A first run on a site that
	// never had a sweep may find more; the rest go on the nights after.
	FilesPerNight = 5000

	// filesPage is how many keys are listed, and judged, at a time.
	filesPage = 500
)

// The folders the sweep judges, each a kind of file.
const (
	folderAvatars      = "avatars"
	folderCovers       = "covers"
	folderPhotos       = "photos"
	folderVideoUploads = "video uploads"
	folderVideos       = "videos"
)

// The folders listed, in the order they are swept. export-service's and
// link-preview-service's are not among them: exports run out by their own
// rule, and a link card's picture may be named by a preview kept in Redis
// for a week, which the sweep cannot see.
var sweptFolders = []struct {
	name   string
	prefix string
}{
	{folderAvatars, assetsPrefix + "identity-image-service/avatars/"},
	{folderCovers, assetsPrefix + "identity-image-service/covers/"},
	{folderPhotos, assetsPrefix + "photo-service/photo/"},
	{folderVideoUploads, videos.UploadsPrefix},
	{folderVideos, videos.StoragePrefix},
}

// storedFile is a key the sweep understood: which folder it is in, and the
// row that should name it.
type storedFile struct {
	key    string
	folder string
	// The node, for an avatar or a cover.
	nodeID int64
	// The file's own name, as its row keeps it: "<name>.<ext>".
	filename string
	// The upload, for a video's source or its files.
	uploadID string
}

var (
	// <kind>/<id>/<name>_<size>.<ext>, under avatars/ or covers/.
	portraitKey = regexp.MustCompile(`^[a-z]+/(\d+)/([^/]+)_[a-z]+\.([A-Za-z0-9]+)$`)
	// <owner id>/<name>_<size>.<ext>, under photo-service/photo/.
	photoKey = regexp.MustCompile(`^\d+/([^/]+)_[a-z]+\.([A-Za-z0-9]+)$`)
	// <person id>/<upload id>/source, under video-service/uploads/.
	videoUploadKey = regexp.MustCompile(`^\d+/([A-Za-z0-9-]+)/source$`)
	// <owner id>/<upload id>/<file>, under video-service/videos/.
	videoFileKey = regexp.MustCompile(`^\d+/([A-Za-z0-9-]+)/[^/]+$`)
)

// parseStoredFile reads a key in one of the swept folders. ok is false for
// a key in no shape this knows, which is kept.
func parseStoredFile(folder, prefix, key string) (storedFile, bool) {
	rest := strings.TrimPrefix(key, prefix)
	if rest == key || strings.Contains(rest, "..") {
		return storedFile{}, false
	}
	file := storedFile{key: key, folder: folder}

	switch folder {
	case folderAvatars, folderCovers:
		found := portraitKey.FindStringSubmatch(rest)
		if found == nil {
			return storedFile{}, false
		}
		id, err := strconv.ParseInt(found[1], 10, 64)
		if err != nil || id <= 0 {
			return storedFile{}, false
		}
		file.nodeID = id
		file.filename = found[2] + "." + found[3]
	case folderPhotos:
		found := photoKey.FindStringSubmatch(rest)
		if found == nil {
			return storedFile{}, false
		}
		file.filename = found[1] + "." + found[2]
	case folderVideoUploads:
		found := videoUploadKey.FindStringSubmatch(rest)
		if found == nil {
			return storedFile{}, false
		}
		file.uploadID = found[1]
	case folderVideos:
		found := videoFileKey.FindStringSubmatch(rest)
		if found == nil {
			return storedFile{}, false
		}
		file.uploadID = found[1]
	default:
		return storedFile{}, false
	}
	return file, true
}

// named is what the database says names a page of files, read once for the
// page. A file is named when its row is here.
type named struct {
	// "<id>/<file name>" for each avatar_filename and cover_filename kept,
	// under the node's own id and under its owner's: a cover's key carries
	// its owner's id when it has one (an article's, its author's), and the
	// node's own otherwise (AnahitaNode.GetCoverURL).
	avatars map[string]bool
	covers  map[string]bool
	// Photo file names in photo_files or photo_uploads.
	photos map[string]bool
	// Uploads video_uploads still has.
	uploads map[string]bool
	// Every key a video post's meta names, read once for the run.
	videoKeys map[string]bool
}

// isNamed says whether a row names a file.
func (n named) isNamed(file storedFile) bool {
	switch file.folder {
	case folderAvatars:
		return n.avatars[portraitPair(file.nodeID, file.filename)]
	case folderCovers:
		return n.covers[portraitPair(file.nodeID, file.filename)]
	case folderPhotos:
		return n.photos[file.filename]
	case folderVideoUploads:
		return n.uploads[file.uploadID]
	case folderVideos:
		// A video's files: named by the post they were made for, or, still
		// being made, by their upload.
		return n.videoKeys[file.key] || n.uploads[file.uploadID]
	}
	return true
}

// FolderCount is what the sweep did in one folder.
type FolderCount struct {
	Seen int
	// Named by a row, or too new to judge.
	Kept int
	// In no shape the sweep knows, or in a page whose query failed.
	Unsure int
	// Removed, or, in a dry run, that would have been.
	Removed int
	// Could not be removed; tried again the next night.
	Failed int
}

// FilesSwept is a run's counts, by folder. Names of files are not logged.
type FilesSwept struct {
	DryRun  bool
	Folders map[string]*FolderCount
}

// String is the counts, one folder after another, for the log.
func (f FilesSwept) String() string {
	names := make([]string, 0, len(f.Folders))
	for name := range f.Folders {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	verb := "removed"
	if f.DryRun {
		verb = "would remove"
	}
	for _, name := range names {
		c := f.Folders[name]
		parts = append(parts, fmt.Sprintf("%s: %d seen, %d kept, %d unsure, %s %d, %d failed",
			name, c.Seen, c.Kept, c.Unsure, verb, c.Removed, c.Failed))
	}
	return strings.Join(parts, "; ")
}

// RunFiles sweeps the stored files once, under the sweeper's lock. With
// dryRun nothing is removed: it counts what would be.
func RunFiles(db *sql.DB, store FileStore, dryRun bool) (FilesSwept, error) {
	swept := FilesSwept{DryRun: dryRun, Folders: map[string]*FolderCount{}}

	acquired, release, err := acquireLock(db, lockTimeout)
	if err != nil {
		return swept, fmt.Errorf("acquiring the sweeper lock: %w", err)
	}
	if !acquired {
		log.Println("sweeper: another run holds the lock, skipping the file sweep")
		return swept, nil
	}
	defer release()

	videoKeys, err := readVideoKeys(db)
	if err != nil {
		// Without them no video file can be judged; the other folders can.
		log.Println("sweeper: video posts not read, their files are kept:", err)
		videoKeys = nil
	}

	cutoff := time.Now().Add(-FileMinAge)
	removed := 0

	for _, folder := range sweptFolders {
		count := &FolderCount{}
		swept.Folders[folder.name] = count
		if folder.name == folderVideos && videoKeys == nil {
			continue
		}

		after := ""
		for {
			page, more, err := store.Browse(folder.prefix, after, filesPage)
			if err != nil {
				return swept, fmt.Errorf("listing %s: %w", folder.prefix, err)
			}
			if len(page) == 0 {
				break
			}
			after = page[len(page)-1].Path

			judged := judgePage(folder.name, folder.prefix, page, cutoff, count)
			n, err := readNamed(db, judged)
			if err != nil {
				log.Printf("sweeper: %s not judged, a page kept: %v", folder.name, err)
				count.Unsure += len(judged)
			} else {
				n.videoKeys = videoKeys
				for _, file := range judged {
					if n.isNamed(file) {
						count.Kept++
						continue
					}
					if removed >= FilesPerNight {
						count.Kept++
						continue
					}
					if !dryRun {
						if err := store.Delete(&services.FileDeleteRequest{Path: file.key}); err != nil {
							count.Failed++
							continue
						}
					}
					count.Removed++
					removed++
				}
			}

			if !more {
				break
			}
		}
	}

	if removed >= FilesPerNight {
		log.Printf("sweeper: %d files a night reached, the rest are left for the nights after", FilesPerNight)
	}
	return swept, nil
}

// judgePage counts a page and returns the files in it the sweep may judge:
// in a shape it knows, and a day old.
func judgePage(folder, prefix string, page []services.BrowsedFile, cutoff time.Time, count *FolderCount) []storedFile {
	judged := make([]storedFile, 0, len(page))
	for _, each := range page {
		count.Seen++
		file, ok := parseStoredFile(folder, prefix, each.Path)
		if !ok {
			count.Unsure++
			continue
		}
		if each.ModifiedAt.IsZero() || each.ModifiedAt.After(cutoff) {
			count.Kept++
			continue
		}
		judged = append(judged, file)
	}
	return judged
}

const (
	// The avatars and covers kept by a page of nodes, or by what they own:
	// a key's id is the node's own or its owner's. A node that is not here
	// is gone: purged.
	nodePortraitsQuery = `
SELECT
	id,
	owner_id,
	avatar_filename,
	cover_filename
FROM nodes
WHERE (id IN (%[1]s) OR owner_id IN (%[1]s))
	AND (avatar_filename <> '' OR cover_filename <> '')`

	// Which of a page of photo file names a post or an upload keeps.
	photoNamesQuery = `
SELECT filename
FROM photo_files
WHERE filename IN (%[1]s)
UNION
SELECT filename
FROM photo_uploads
WHERE filename IN (%[1]s)`

	// Which of a page of video uploads are still there.
	videoUploadsQuery = `
SELECT id
FROM video_uploads
WHERE id IN (%s)`

	// Every video post's meta, for the files it names.
	videoPostsQuery = `
SELECT meta
FROM nodes
WHERE type = ?`
)

// readNamed asks the database, once for the page, what names its files.
func readNamed(db *sql.DB, page []storedFile) (named, error) {
	n := named{
		avatars: map[string]bool{},
		covers:  map[string]bool{},
		photos:  map[string]bool{},
		uploads: map[string]bool{},
	}

	var nodeIDs []any
	var filenames []any
	var uploadIDs []any
	seenNode := map[int64]bool{}
	seenName := map[string]bool{}
	seenUpload := map[string]bool{}
	for _, file := range page {
		switch {
		case file.nodeID > 0 && !seenNode[file.nodeID]:
			seenNode[file.nodeID] = true
			nodeIDs = append(nodeIDs, file.nodeID)
		case file.filename != "" && file.nodeID == 0 && !seenName[file.filename]:
			seenName[file.filename] = true
			filenames = append(filenames, file.filename)
		case file.uploadID != "" && !seenUpload[file.uploadID]:
			seenUpload[file.uploadID] = true
			uploadIDs = append(uploadIDs, file.uploadID)
		}
	}

	if len(nodeIDs) > 0 {
		args := append(append([]any{}, nodeIDs...), nodeIDs...)
		rows, err := db.Query(fmt.Sprintf(nodePortraitsQuery, placeholders(len(nodeIDs))), args...)
		if err != nil {
			return n, err
		}
		for rows.Next() {
			var id int64
			var owner sql.NullInt64
			var avatar, cover string
			if err := rows.Scan(&id, &owner, &avatar, &cover); err != nil {
				rows.Close()
				return n, err
			}
			for _, under := range []int64{id, owner.Int64} {
				if under <= 0 {
					continue
				}
				if avatar != "" {
					n.avatars[portraitPair(under, avatar)] = true
				}
				if cover != "" {
					n.covers[portraitPair(under, cover)] = true
				}
			}
		}
		if err := closeRows(rows); err != nil {
			return n, err
		}
	}

	if len(filenames) > 0 {
		args := append(append([]any{}, filenames...), filenames...)
		rows, err := db.Query(fmt.Sprintf(photoNamesQuery, placeholders(len(filenames))), args...)
		if err != nil {
			return n, err
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return n, err
			}
			n.photos[name] = true
		}
		if err := closeRows(rows); err != nil {
			return n, err
		}
	}

	if len(uploadIDs) > 0 {
		rows, err := db.Query(fmt.Sprintf(videoUploadsQuery, placeholders(len(uploadIDs))), uploadIDs...)
		if err != nil {
			return n, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return n, err
			}
			n.uploads[id] = true
		}
		if err := closeRows(rows); err != nil {
			return n, err
		}
	}

	return n, nil
}

// readVideoKeys is every file every video post names, its sizes and its
// poster, read once for the run.
func readVideoKeys(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(videoPostsQuery, videos.NodeType)
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for rows.Next() {
		var meta []byte
		if err := rows.Scan(&meta); err != nil {
			rows.Close()
			return nil, err
		}
		for _, key := range videos.FromMeta(meta).Keys() {
			keys[key] = true
		}
	}
	return keys, closeRows(rows)
}

// portraitPair is a picture's file name under the id its key carries.
func portraitPair(id int64, filename string) string {
	return strconv.FormatInt(id, 10) + "/" + filename
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func closeRows(rows *sql.Rows) error {
	err := rows.Err()
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	return err
}
