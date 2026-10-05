-- The images of a photo post, in order. A post holds up to four.
--
-- One post with several images, not an album of posts: one caption, one set
-- of likes and replies, one audience. It is also how the fediverse sends
-- them, as one post with attachments.
--
-- nodes.filename and nodes.mimetype stay, as a copy of the first image, so
-- everything that shows one image for a photo (feeds, search, notifications)
-- goes on working without knowing about this table.
--
-- photo_uploads holds images that have been uploaded and are not in a post
-- yet. Each image is uploaded in a request of its own, which keeps every
-- request small, and the post is then made from them. An upload nobody uses
-- within a day is removed, with its stored images, by the sweeper.
--
-- Both owned by graph-grpc-service. No foreign keys, like the rest of this
-- schema.
--
-- CREATE TABLE IF NOT EXISTS, for the same reason the migrations before it
-- are guarded: entrypoint.sh runs schema/init.sql on an empty database and
-- stamps the versions afterwards, so a migration must survive being run
-- against a schema that already has its change.

-- +goose Up
CREATE TABLE IF NOT EXISTS photo_files (
    -- The photo post.
    `node_id` bigint unsigned NOT NULL,
    -- Where it comes in the post, from 0. The first is the cover.
    `position` tinyint unsigned NOT NULL,
    -- The stored name, as nodes.filename holds it for a single image. Each
    -- size is stored under this name with the size added.
    `filename` varchar(255) NOT NULL,
    `mimetype` varchar(100) NOT NULL DEFAULT '',
    -- Of the image as uploaded, once turned the right way up. 0 for an
    -- image stored before this table existed.
    `width` int unsigned NOT NULL DEFAULT 0,
    `height` int unsigned NOT NULL DEFAULT 0,
    -- What the image shows, for people who cannot see it.
    `alt_text` varchar(1500) NOT NULL DEFAULT '',
    PRIMARY KEY (`node_id`, `position`),
    UNIQUE KEY `uq_photo_files_filename` (`filename`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS photo_uploads (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- The profile it was uploaded for. Part of where the image is stored.
    `owner_id` bigint unsigned NOT NULL,
    -- Who uploaded it. Nobody else can put it in a post.
    `created_by` bigint unsigned NOT NULL,
    `filename` varchar(255) NOT NULL,
    `mimetype` varchar(100) NOT NULL DEFAULT '',
    `width` int unsigned NOT NULL DEFAULT 0,
    `height` int unsigned NOT NULL DEFAULT 0,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_photo_uploads_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Every photo there already is has one image, the one nodes.filename names.
-- INSERT IGNORE, so running this twice adds nothing.
INSERT IGNORE INTO photo_files (node_id, position, filename, mimetype)
SELECT
  id,
  0,
  filename,
  COALESCE(mimetype, '')
FROM nodes
WHERE type = 'node.medium.photo-service.photo.v1'
  AND filename IS NOT NULL
  AND filename <> '';

-- +goose Down
DROP TABLE IF EXISTS photo_uploads;
DROP TABLE IF EXISTS photo_files;
