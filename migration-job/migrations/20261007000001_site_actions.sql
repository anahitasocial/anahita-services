-- A record of the things a super administrator does to the whole
-- installation at once, which no single profile's history would show.
--
-- The first is converting everything public to registered. It changes
-- thousands of rows in one step and cannot be undone, so who did it and what
-- it came to is written down here, once per action.
--
-- Owned by graph-grpc-service.
--
-- CREATE TABLE IF NOT EXISTS, for the same reason the migrations before it
-- are guarded: entrypoint.sh runs schema/init.sql on an empty database and
-- stamps the versions afterwards, so a migration must survive being run
-- against a schema that already has its change.

-- +goose Up
CREATE TABLE IF NOT EXISTS site_actions (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- What was done, as a short fixed word: 'made-public-registered'.
    `action` varchar(64) NOT NULL,
    -- The super administrator who did it.
    `performed_by` bigint unsigned NOT NULL,
    -- What it came to, as JSON: how many rows of each kind it changed.
    `detail` text,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_site_actions_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS site_actions;
