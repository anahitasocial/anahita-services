-- The queue of accounts a super administrator has asked to be removed now.
--
-- Asking does not purge. It hides the account at once and adds a row here; the
-- purge itself is done by whoever holds the sweeper's lock, one account at a
-- time, so a request for five hundred accounts returns immediately and cannot
-- time out. The rows are also the progress of the request, and afterwards the
-- record of who asked for what.
--
-- Owned by graph-grpc-service.
--
-- CREATE TABLE IF NOT EXISTS, for the same reason the migrations before it
-- are guarded: entrypoint.sh runs schema/init.sql on an empty database and
-- stamps the versions afterwards, so a migration must survive being run
-- against a schema that already has its change.

-- +goose Up
CREATE TABLE IF NOT EXISTS purge_requests (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- The request that asked for it. One request names many actors, and its
    -- progress is the state of these rows.
    `batch_id` char(36) NOT NULL,
    `actor_id` bigint unsigned NOT NULL,
    -- Kept here so the progress view can still name an account after the
    -- purge has taken its name.
    `actor_alias` varchar(255) NOT NULL DEFAULT '',
    `actor_type` varchar(255) NOT NULL DEFAULT '',
    -- 'queued', 'done' or 'failed'.
    `status` varchar(16) NOT NULL DEFAULT 'queued',
    -- How many times the purge has been tried. It is given up as 'failed'
    -- after a few, with the last error kept.
    `attempts` int unsigned NOT NULL DEFAULT 0,
    `error` varchar(255) NOT NULL DEFAULT '',
    -- The super administrator who asked.
    `requested_by` bigint unsigned NOT NULL,
    `requested_at` datetime NOT NULL,
    `finished_at` datetime DEFAULT NULL,
    PRIMARY KEY (`id`),
    -- An actor is purged once. A failed request is replaced by asking again.
    UNIQUE KEY `idx_purge_requests_actor` (`actor_id`),
    KEY `idx_purge_requests_batch` (`batch_id`),
    KEY `idx_purge_requests_status` (`status`, `requested_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS purge_requests;
