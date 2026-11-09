// Package repositories keeps the uploads: a video from the moment a
// browser starts to send it until it is a post, or is given up on.
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	an_const "anahita-libs/constants"
	"anahita-libs/videos"

	"video/constants"
)

// Upload is one row of video_uploads.
type Upload struct {
	ID       string
	PersonID int64
	OwnerID  int64
	// What the worker makes of it: videos.KindVideo or videos.KindAudio.
	Kind          string
	Name          string
	Body          string
	Access        string
	Language      string
	ReplyAccess   string
	QuotePolicy   string
	AllowDownload bool
	// Its content warning, as chosen when it began.
	Warning     string
	WarningText string
	LibraryOnly bool
	// What it answers when it is a reply; 0 for a post.
	ParentID int64
	// In a thread written in one go (roadmap row 143): the upload whose post
	// this one waits to answer the thread after, "" for none; it is not
	// made until ParentID is set, by the worker that made that one. And the
	// notes after this one, up to the next upload, as JSON (threads.Rest).
	AfterUploadID   string
	ThreadAfter     string
	Status          string
	SourceKey       string
	StorageUploadID string
	DeclaredBytes   int64
	SizeBytes       int64
	NodeID          int64
	Error           string
	Attempts        int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ErrNotInState is a change refused because the upload has moved on: it
// was cancelled while a worker had it, or finished twice.
var ErrNotInState = errors.New("the upload is no longer in that state")

const uploadColumns = `
    id,
    person_id,
    owner_id,
    kind,
    name,
    body,
    access,
    language,
    reply_access,
    quote_policy,
    allow_download,
    warning,
    warning_text,
    library_only,
    COALESCE(parent_id, 0),
    COALESCE(after_upload_id, ''),
    thread_after,
    status,
    source_key,
    storage_upload_id,
    declared_bytes,
    size_bytes,
    COALESCE(node_id, 0),
    error,
    attempts,
    created_at,
    updated_at`

const uploadAdd = `
INSERT INTO video_uploads (
    id,
    person_id,
    owner_id,
    kind,
    name,
    body,
    access,
    language,
    reply_access,
    quote_policy,
    allow_download,
    warning,
    warning_text,
    library_only,
    parent_id,
    after_upload_id,
    thread_after,
    status,
    source_key,
    storage_upload_id,
    declared_bytes,
    created_at,
    updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)`

const uploadRead = `
SELECT` + uploadColumns + `
FROM video_uploads
WHERE id = ?`

// What somebody has in hand: still being sent, being made, or failed in
// the last day. Not what is finished, which is a post.
const uploadsOfPerson = `
SELECT` + uploadColumns + `
FROM video_uploads
WHERE person_id = ?
  AND (
      status IN ('uploading', 'processing')
      OR (status = 'failed' AND updated_at > ?)
  )
ORDER BY created_at DESC
LIMIT 20`

// Every change of state names the state it changes from, so that two
// things changing one upload at once cannot both succeed.
const uploadToProcessing = `
UPDATE video_uploads
SET status = 'processing',
    size_bytes = ?,
    storage_upload_id = '',
    updated_at = ?
WHERE id = ?
  AND status = 'uploading'`

const uploadToCancelled = `
UPDATE video_uploads
SET status = 'cancelled',
    updated_at = ?
WHERE id = ?
  AND status IN ('uploading', 'processing', 'failed')`

const uploadToReady = `
UPDATE video_uploads
SET status = 'ready',
    node_id = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE id = ?
  AND status = 'processing'`

const uploadToFailed = `
UPDATE video_uploads
SET status = 'failed',
    error = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE id = ?
  AND status IN ('uploading', 'processing')`

// Let go of by a worker that could not finish, for another try later.
const uploadRelease = `
UPDATE video_uploads
SET claimed_at = NULL,
    updated_at = ?
WHERE id = ?
  AND status = 'processing'`

// The next video waiting for a worker: never taken, or taken so long ago
// that whoever had it must have died. SKIP LOCKED lets several workers ask
// at once without waiting on each other or taking the same one.
const uploadNextWaiting = `
SELECT id
FROM video_uploads
WHERE status = 'processing'
  AND attempts < ?
  AND (claimed_at IS NULL OR claimed_at < ?)
  -- One that waits in a thread for the upload before it, until that one's
  -- post and the notes after it are made.
  AND (after_upload_id IS NULL OR parent_id IS NOT NULL)
ORDER BY updated_at
LIMIT 1
FOR UPDATE SKIP LOCKED`

const uploadClaim = `
UPDATE video_uploads
SET claimed_at = ?,
    attempts = attempts + 1
WHERE id = ?`

// Tried as often as it will be, and still not done.
const uploadsWornOut = `
SELECT id
FROM video_uploads
WHERE status = 'processing'
  AND attempts >= ?
  AND (claimed_at IS NULL OR claimed_at < ?)
LIMIT 20`

// The upload that waits for this one in a thread is let go: it answers
// the last note made after this one's post.
const uploadsFollowOn = `
UPDATE video_uploads
SET parent_id = ?,
    updated_at = ?
WHERE after_upload_id = ?
  AND status IN ('uploading', 'processing')`

// An upload whose thread stopped before it, because the upload it waits
// for failed or was given up, stops too.
const uploadsThreadsStopped = `
UPDATE video_uploads AS waiting
JOIN video_uploads AS before_it ON waiting.after_upload_id = before_it.id
SET waiting.status = 'failed',
    waiting.error = ?,
    waiting.claimed_at = NULL,
    waiting.updated_at = ?
WHERE waiting.status IN ('uploading', 'processing')
  AND (before_it.status IN ('failed', 'cancelled')
       OR (before_it.status = 'ready' AND waiting.parent_id IS NULL))`

const uploadsAbandoned = `
SELECT` + uploadColumns + `
FROM video_uploads
WHERE status = 'uploading'
  AND created_at < ?
LIMIT 50`

const uploadsSince = `
SELECT COUNT(*)
FROM video_uploads
WHERE person_id = ?
  AND created_at > ?`

const uploadsForget = `
DELETE FROM video_uploads
WHERE status IN ('ready', 'cancelled', 'failed')
  AND updated_at < ?`

type UploadsInterface interface {
	Add(upload *Upload) error
	Read(id string) (*Upload, error)
	OfPerson(personID int64) ([]*Upload, error)
	CountSince(personID int64, since time.Time) (int, error)
	ToProcessing(id string, size int64) error
	ToCancelled(id string) error
	ToReady(id string, nodeID int64) error
	ToFailed(id, reason string) error
	Release(id string) error
	// FollowOn lets go the upload that waits for this one in a thread, to
	// answer parentID.
	FollowOn(id string, parentID int64) error
	// StopThreads fails every upload whose thread stopped before it, and
	// says how many.
	StopThreads(reason string) (int64, error)
	// Claim takes the next video waiting, or returns nil when none is.
	Claim() (*Upload, error)
	WornOut() ([]string, error)
	Abandoned() ([]*Upload, error)
	Forget(before time.Time) error
}

type UploadsRepo struct {
	db      *sql.DB
	timeout time.Duration
	now     func() time.Time
	// How long a worker may hold a video before another takes it over:
	// longer than the work on one is allowed to take.
	ClaimFor time.Duration
}

func NewUploads(db *sql.DB) *UploadsRepo {
	return &UploadsRepo{db: db, timeout: an_const.DBTimeout, now: func() time.Time { return time.Now().UTC() }, ClaimFor: 45 * time.Minute}
}

func (r *UploadsRepo) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), r.timeout)
}

func (r *UploadsRepo) Add(upload *Upload) error {
	ctx, cancel := r.ctx()
	defer cancel()

	now := r.now()
	upload.Status = videos.StatusUploading
	upload.CreatedAt, upload.UpdatedAt = now, now
	if upload.Kind == "" {
		upload.Kind = videos.KindVideo
	}

	_, err := r.db.ExecContext(ctx, uploadAdd,
		upload.ID, upload.PersonID, upload.OwnerID, upload.Kind, upload.Name, upload.Body,
		upload.Access, upload.Language, upload.ReplyAccess, upload.QuotePolicy,
		upload.AllowDownload, upload.Warning, upload.WarningText, upload.LibraryOnly, upload.ParentID, upload.AfterUploadID, upload.ThreadAfter, upload.Status, upload.SourceKey, upload.StorageUploadID, upload.DeclaredBytes,
		now, now)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUpload(row scanner) (*Upload, error) {
	u := &Upload{}
	err := row.Scan(
		&u.ID, &u.PersonID, &u.OwnerID, &u.Kind, &u.Name, &u.Body, &u.Access, &u.Language,
		&u.ReplyAccess, &u.QuotePolicy, &u.AllowDownload, &u.Warning, &u.WarningText, &u.LibraryOnly, &u.ParentID, &u.AfterUploadID, &u.ThreadAfter, &u.Status, &u.SourceKey, &u.StorageUploadID,
		&u.DeclaredBytes, &u.SizeBytes, &u.NodeID, &u.Error, &u.Attempts,
		&u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Read returns an upload, or nil when there is none by that id.
func (r *UploadsRepo) Read(id string) (*Upload, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	upload, err := scanUpload(r.db.QueryRowContext(ctx, uploadRead, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return upload, err
}

func (r *UploadsRepo) list(query string, args ...any) ([]*Upload, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var uploads []*Upload
	for rows.Next() {
		upload, err := scanUpload(rows)
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, upload)
	}
	return uploads, rows.Err()
}

func (r *UploadsRepo) OfPerson(personID int64) ([]*Upload, error) {
	return r.list(uploadsOfPerson, personID, r.now().Add(-24*time.Hour))
}

func (r *UploadsRepo) Abandoned() ([]*Upload, error) {
	return r.list(uploadsAbandoned, r.now().Add(-constants.AbandonedAfterHours*time.Hour))
}

func (r *UploadsRepo) CountSince(personID int64, since time.Time) (int, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	var count int
	err := r.db.QueryRowContext(ctx, uploadsSince, personID, since).Scan(&count)
	return count, err
}

// change runs one change of state and says whether it took.
func (r *UploadsRepo) change(query string, args ...any) error {
	ctx, cancel := r.ctx()
	defer cancel()

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return err
	} else if changed == 0 {
		return ErrNotInState
	}
	return nil
}

func (r *UploadsRepo) ToProcessing(id string, size int64) error {
	return r.change(uploadToProcessing, size, r.now(), id)
}

func (r *UploadsRepo) ToCancelled(id string) error {
	return r.change(uploadToCancelled, r.now(), id)
}

func (r *UploadsRepo) ToReady(id string, nodeID int64) error {
	return r.change(uploadToReady, nodeID, r.now(), id)
}

func (r *UploadsRepo) ToFailed(id, reason string) error {
	return r.change(uploadToFailed, reason, r.now(), id)
}

func (r *UploadsRepo) Release(id string) error {
	return r.change(uploadRelease, r.now(), id)
}

func (r *UploadsRepo) Claim() (*Upload, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	now := r.now()
	stale := now.Add(-r.ClaimFor)

	var id string
	err = tx.QueryRowContext(ctx, uploadNextWaiting, constants.MaxAttempts, stale).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, uploadClaim, now, id); err != nil {
		return nil, err
	}
	upload, err := scanUpload(tx.QueryRowContext(ctx, uploadRead, id))
	if err != nil {
		return nil, err
	}

	return upload, tx.Commit()
}

func (r *UploadsRepo) WornOut() ([]string, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	stale := r.now().Add(-r.ClaimFor)
	rows, err := r.db.QueryContext(ctx, uploadsWornOut, constants.MaxAttempts, stale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *UploadsRepo) Forget(before time.Time) error {
	ctx, cancel := r.ctx()
	defer cancel()

	_, err := r.db.ExecContext(ctx, uploadsForget, before)
	return err
}

func (r *UploadsRepo) FollowOn(id string, parentID int64) error {
	ctx, cancel := r.ctx()
	defer cancel()
	_, err := r.db.ExecContext(ctx, uploadsFollowOn, parentID, r.now(), id)
	return err
}

// StopThreads runs until nothing is left to stop, a thread of several
// uploads stopping one upload a round.
func (r *UploadsRepo) StopThreads(reason string) (int64, error) {
	var all int64
	for round := 0; round < 20; round++ {
		ctx, cancel := r.ctx()
		res, err := r.db.ExecContext(ctx, uploadsThreadsStopped, reason, r.now())
		cancel()
		if err != nil {
			return all, err
		}
		n, _ := res.RowsAffected()
		all += n
		if n == 0 {
			break
		}
	}
	return all, nil
}
