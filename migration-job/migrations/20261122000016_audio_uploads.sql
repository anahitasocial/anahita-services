-- +goose Up
-- Audio posts (anahita-web roadmap row 137): an upload says what the
-- worker makes of it, a video or an audio post.
ALTER TABLE video_uploads
    ADD COLUMN `kind` varchar(8) NOT NULL DEFAULT 'video' AFTER `owner_id`;

-- +goose Down
ALTER TABLE video_uploads
    DROP COLUMN `kind`;
