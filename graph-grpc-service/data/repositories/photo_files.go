package repositories

import (
	"anahita-libs/data/entities"
	"context"
	"database/sql"
	"strings"
	"time"
)

// PhotoFilesRepoInterface keeps the images of photo posts, and the images
// uploaded that are not in a post yet.
type PhotoFilesRepoInterface interface {
	// AddUpload records an image that has been stored and belongs to no post
	// yet, and returns its id.
	AddUpload(upload *entities.PhotoUpload) (int64, error)

	// ReadUploads returns the uploads with these ids that still exist.
	ReadUploads(ids []int64) ([]*entities.PhotoUpload, error)

	// ExpiredUploads returns uploads made before the given time, oldest
	// first.
	ExpiredUploads(before time.Time, limit int) ([]*entities.PhotoUpload, error)

	// DeleteUploads forgets these uploads. The stored images are the
	// caller's to remove.
	DeleteUploads(ids []int64) error

	// Browse returns the images of these posts, each post's in order.
	Browse(nodeIDs []int64) ([]*entities.PhotoFile, error)

	// Set makes these the images of a post, in the order given, and forgets
	// the uploads they were made from. The post's own filename and mimetype
	// are set to the first image's.
	Set(nodeID int64, files []*entities.PhotoFile, uploadIDs []int64) error

	// DeleteForNodes forgets the images of these posts.
	DeleteForNodes(nodeIDs []int64) error
}

func NewPhotoFilesRepo(db *sql.DB) PhotoFilesRepoInterface {
	return &photoFilesRepo{db: db}
}

type photoFilesRepo struct {
	db *sql.DB
}

const insertPhotoUpload = `
INSERT INTO photo_uploads (
  owner_id,
  created_by,
  filename,
  mimetype,
  width,
  height,
  created_at
)
VALUES (?, ?, ?, ?, ?, ?, UTC_TIMESTAMP())`

func (r *photoFilesRepo) AddUpload(upload *entities.PhotoUpload) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	result, err := r.db.ExecContext(ctx, insertPhotoUpload,
		upload.OwnerID,
		upload.CreatedBy,
		upload.Filename,
		upload.Mimetype,
		upload.Width,
		upload.Height,
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

const selectPhotoUploads = `
SELECT
  id,
  owner_id,
  created_by,
  filename,
  mimetype,
  width,
  height
FROM photo_uploads`

func (r *photoFilesRepo) ReadUploads(ids []int64) ([]*entities.PhotoUpload, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := selectPhotoUploads + `
WHERE id IN (` + placeholders(len(ids)) + `)
ORDER BY id`

	return r.queryUploads(query, int64Args(ids)...)
}

func (r *photoFilesRepo) ExpiredUploads(before time.Time, limit int) ([]*entities.PhotoUpload, error) {
	query := selectPhotoUploads + `
WHERE created_at < ?
ORDER BY id
LIMIT ?`

	return r.queryUploads(query, before.UTC(), limit)
}

func (r *photoFilesRepo) queryUploads(query string, args ...interface{}) ([]*entities.PhotoUpload, error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var uploads []*entities.PhotoUpload
	for rows.Next() {
		upload := &entities.PhotoUpload{}
		if err := rows.Scan(
			&upload.ID,
			&upload.OwnerID,
			&upload.CreatedBy,
			&upload.Filename,
			&upload.Mimetype,
			&upload.Width,
			&upload.Height,
		); err != nil {
			return nil, err
		}
		uploads = append(uploads, upload)
	}

	return uploads, rows.Err()
}

func (r *photoFilesRepo) DeleteUploads(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	query := `
DELETE FROM photo_uploads
WHERE id IN (` + placeholders(len(ids)) + `)`

	_, err := r.db.ExecContext(ctx, query, int64Args(ids)...)
	return err
}

func (r *photoFilesRepo) Browse(nodeIDs []int64) ([]*entities.PhotoFile, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	query := `
SELECT
  node_id,
  position,
  filename,
  mimetype,
  width,
  height,
  alt_text
FROM photo_files
WHERE node_id IN (` + placeholders(len(nodeIDs)) + `)
ORDER BY node_id, position`

	rows, err := r.db.QueryContext(ctx, query, int64Args(nodeIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*entities.PhotoFile
	for rows.Next() {
		file := &entities.PhotoFile{}
		if err := rows.Scan(
			&file.NodeID,
			&file.Position,
			&file.Filename,
			&file.Mimetype,
			&file.Width,
			&file.Height,
			&file.AltText,
		); err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	return files, rows.Err()
}

const deletePhotoFilesOfNode = `
DELETE FROM photo_files
WHERE node_id = ?`

const insertPhotoFile = `
INSERT INTO photo_files (
  node_id,
  position,
  filename,
  mimetype,
  width,
  height,
  alt_text
)
VALUES (?, ?, ?, ?, ?, ?, ?)`

// The post's own copy of its first image, which is what everything that
// shows one image for a photo reads. updated_at is left alone here: the
// caller edits the post when the change is one its author made.
const copyFirstPhotoFile = `
UPDATE nodes
SET filename = ?,
    mimetype = ?
WHERE id = ?`

// Set is one transaction, so a post never has half its images, and an upload
// is never both an upload and an image of a post.
//
// A filename is unique among the images of all posts. Two requests using the
// same upload at once therefore cannot both succeed: the second fails on the
// key and changes nothing.
func (r *photoFilesRepo) Set(nodeID int64, files []*entities.PhotoFile, uploadIDs []int64) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, deletePhotoFilesOfNode, nodeID); err != nil {
		return err
	}

	for position, file := range files {
		if _, err = tx.ExecContext(ctx, insertPhotoFile,
			nodeID,
			position,
			file.Filename,
			file.Mimetype,
			file.Width,
			file.Height,
			file.AltText,
		); err != nil {
			return err
		}
	}

	if len(uploadIDs) > 0 {
		query := `
DELETE FROM photo_uploads
WHERE id IN (` + placeholders(len(uploadIDs)) + `)`

		if _, err = tx.ExecContext(ctx, query, int64Args(uploadIDs)...); err != nil {
			return err
		}
	}

	var filename, mimetype string
	if len(files) > 0 {
		filename = files[0].Filename
		mimetype = files[0].Mimetype
	}
	if _, err = tx.ExecContext(ctx, copyFirstPhotoFile, filename, mimetype, nodeID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *photoFilesRepo) DeleteForNodes(nodeIDs []int64) error {
	if len(nodeIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	query := `
DELETE FROM photo_files
WHERE node_id IN (` + placeholders(len(nodeIDs)) + `)`

	_, err := r.db.ExecContext(ctx, query, int64Args(nodeIDs)...)
	return err
}

// placeholders is "?, ?, ?" for n values.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func int64Args(ids []int64) []interface{} {
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}
