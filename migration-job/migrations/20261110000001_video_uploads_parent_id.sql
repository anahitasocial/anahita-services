-- Replies of any kind: a video, or a GIF made into one, can answer a post.
--
--   video_uploads.parent_id  what the video answers, when it is a reply:
--                            chosen when the upload begins, and given to the
--                            post the worker makes. NULL for a post.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_video_uploads_parent_id()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'video_uploads' AND COLUMN_NAME = 'parent_id'
    ) THEN
        ALTER TABLE video_uploads ADD COLUMN `parent_id` bigint unsigned NULL DEFAULT NULL AFTER `allow_download`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_video_uploads_parent_id();
DROP PROCEDURE anahita_add_video_uploads_parent_id;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_video_uploads_parent_id()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'video_uploads' AND COLUMN_NAME = 'parent_id'
    ) THEN
        ALTER TABLE video_uploads DROP COLUMN `parent_id`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_video_uploads_parent_id();
DROP PROCEDURE anahita_drop_video_uploads_parent_id;
