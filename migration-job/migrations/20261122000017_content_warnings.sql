-- +goose Up
-- Content warnings (roadmap row 122): a post's kind of warning, the words
-- of an Other warning, and whether a moderator gave it; and the same on a
-- video still being uploaded, as its author chose when it began.
ALTER TABLE nodes
    ADD COLUMN `warning` varchar(16) NOT NULL DEFAULT '' AFTER `draft`,
    ADD COLUMN `warning_text` varchar(100) NOT NULL DEFAULT '' AFTER `warning`,
    ADD COLUMN `warning_by_moderator` tinyint(1) NOT NULL DEFAULT 0 AFTER `warning_text`;

ALTER TABLE video_uploads
    ADD COLUMN `warning` varchar(16) NOT NULL DEFAULT '' AFTER `allow_download`,
    ADD COLUMN `warning_text` varchar(100) NOT NULL DEFAULT '' AFTER `warning`;

-- +goose Down
ALTER TABLE video_uploads
    DROP COLUMN `warning_text`,
    DROP COLUMN `warning`;

ALTER TABLE nodes
    DROP COLUMN `warning_by_moderator`,
    DROP COLUMN `warning_text`,
    DROP COLUMN `warning`;
