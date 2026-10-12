-- Downloading what a post holds: a photo, a video.
--
--   nodes.allow_download          on a post that holds a file: whether
--                                 whoever can see it may download it. On a
--                                 person or a group: what their new posts
--                                 start with.
--   video_uploads.allow_download  the same choice, made when a video's
--                                 upload begins and carried to its post.
--
-- Off for everything there is: nobody is taken to have agreed.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_allow_download()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'allow_download'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `allow_download` tinyint(1) NOT NULL DEFAULT 0 AFTER `show_reposts_on_profile`;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'video_uploads' AND COLUMN_NAME = 'allow_download'
    ) THEN
        ALTER TABLE video_uploads ADD COLUMN `allow_download` tinyint(1) NOT NULL DEFAULT 0 AFTER `quote_policy`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_allow_download();
DROP PROCEDURE anahita_add_allow_download;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_allow_download()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'video_uploads' AND COLUMN_NAME = 'allow_download'
    ) THEN
        ALTER TABLE video_uploads DROP COLUMN `allow_download`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'allow_download'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `allow_download`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_allow_download();
DROP PROCEDURE anahita_drop_allow_download;
