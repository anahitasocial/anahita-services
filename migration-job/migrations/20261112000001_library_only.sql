-- A photo or video post that is in its author's media library only: put up
-- from inside the article editor, to be shown in an article, and not
-- announced. It is not in any feed, a profile's lists, search, a hashtag's
-- or a place's list, and nothing is sent of it to other servers. It is
-- read by its address with the access any post has, and listed for its
-- author in the library. (The user's choice, 2026-10-16.)
--
--   nodes.library_only          on a post: whether it is so.
--   video_uploads.library_only  the same, chosen when a video's upload
--                               begins and carried to its post.
--
-- Off for everything there is.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_library_only()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'library_only'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `library_only` tinyint(1) NOT NULL DEFAULT 0 AFTER `allow_download`;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'video_uploads' AND COLUMN_NAME = 'library_only'
    ) THEN
        ALTER TABLE video_uploads ADD COLUMN `library_only` tinyint(1) NOT NULL DEFAULT 0 AFTER `allow_download`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_library_only();
DROP PROCEDURE anahita_add_library_only;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_library_only()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'video_uploads' AND COLUMN_NAME = 'library_only'
    ) THEN
        ALTER TABLE video_uploads DROP COLUMN `library_only`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'library_only'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `library_only`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_library_only();
DROP PROCEDURE anahita_drop_library_only;
