-- +goose Up
-- GIFs, videos and sounds in a thread's notes (roadmap row 143, phase 2):
-- the notes after one wait with its upload until its file is ready, and a
-- later one in the same thread waits for the upload before it.
ALTER TABLE video_uploads
    ADD COLUMN `after_upload_id` char(36) NULL DEFAULT NULL AFTER `parent_id`,
    ADD COLUMN `thread_after` mediumtext NOT NULL DEFAULT ('') AFTER `after_upload_id`,
    ADD KEY `idx_video_uploads_after` (`after_upload_id`);

-- +goose Down
ALTER TABLE video_uploads
    DROP KEY `idx_video_uploads_after`,
    DROP COLUMN `thread_after`,
    DROP COLUMN `after_upload_id`;
