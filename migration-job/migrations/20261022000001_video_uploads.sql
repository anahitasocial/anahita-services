-- Videos: an upload on its way to being a post.
--
-- GUARDED by IF NOT EXISTS: entrypoint.sh runs schema/init.sql on an empty
-- database and stamps the versions afterwards, so a migration must survive
-- being run against a schema that already has its change.

-- +goose Up
-- A video on its way to being a post: sent by a browser straight to
-- storage, then made into the files that are shown. The row is the upload,
-- the work waiting to be done on it, and afterwards the record of how it
-- went; the post itself is a node, made only once there is something to
-- show.
--
-- Owned by video-service and its worker. No foreign keys, like the rest of
-- this schema.
CREATE TABLE IF NOT EXISTS video_uploads (
    -- Public, and unguessable: it is in the addresses the uploader asks.
    `id` char(36) NOT NULL,
    -- Who is posting it, and the profile it is posted on.
    `person_id` bigint unsigned NOT NULL,
    `owner_id` bigint unsigned NOT NULL,
    -- The post as it will be made: said when the upload began, and settled
    -- then, so that nothing has to be asked again when the video is ready.
    `name` varchar(255) NOT NULL DEFAULT '',
    `body` text,
    `access` varchar(32) NOT NULL DEFAULT '',
    `language` varchar(50) NOT NULL DEFAULT '',
    `reply_access` varchar(64) NOT NULL DEFAULT '',
    `quote_policy` varchar(16) NOT NULL DEFAULT '',
    -- 'uploading' while the browser sends it, 'processing' once it is all
    -- there and waiting for or with a worker, then 'ready', 'failed', or
    -- 'cancelled' for one given up on.
    `status` varchar(16) NOT NULL DEFAULT 'uploading',
    -- Where the file sent is in storage, and the store's own name for the
    -- upload while it is in parts.
    `source_key` varchar(255) NOT NULL,
    `storage_upload_id` varchar(255) NOT NULL DEFAULT '',
    -- What the uploader said it was, and what arrived.
    `declared_bytes` bigint unsigned NOT NULL DEFAULT 0,
    `size_bytes` bigint unsigned NOT NULL DEFAULT 0,
    -- The post, once there is one.
    `node_id` bigint unsigned DEFAULT NULL,
    -- Why it failed, in a word the uploader is shown the meaning of.
    `error` varchar(64) NOT NULL DEFAULT '',
    -- How many times a worker has taken it, and when one last did. A worker
    -- that dies leaves it taken; it is taken again after a while.
    `attempts` int NOT NULL DEFAULT 0,
    `claimed_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_video_uploads_status` (`status`, `updated_at`),
    KEY `idx_video_uploads_person` (`person_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS video_uploads;
