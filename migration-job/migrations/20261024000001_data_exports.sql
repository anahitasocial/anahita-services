-- Data exports: a person's copy of their own data.
--
-- GUARDED by IF NOT EXISTS, for the same reason as the migrations before it.

-- +goose Up
-- A person's copy of their own data: one archive of what they wrote and
-- who they follow, made when they ask and kept a week for them to fetch.
-- The row is the request, the work to be done on it, and the record of it.
--
-- Owned by export-service and its worker. No foreign keys, like the rest of
-- this schema.
CREATE TABLE IF NOT EXISTS data_exports (
    -- Public, and unguessable: it is in the addresses its owner asks.
    `id` char(36) NOT NULL,
    `person_id` bigint unsigned NOT NULL,
    -- 'pending' until a worker takes it, 'building' while one has it, then
    -- 'ready', 'failed', 'cancelled' for one given up, or 'expired' once
    -- its archive has been deleted.
    `status` varchar(16) NOT NULL DEFAULT 'pending',
    -- The person, while the export is being made, and NULL afterwards.
    -- Unique, which is what makes "one at a time for each person" a rule of
    -- the database and not of whichever request got there first. Any number
    -- of finished exports store NULL and share the index.
    `active_person_id` bigint unsigned GENERATED ALWAYS AS (IF(`status` IN ('pending', 'building'), `person_id`, NULL)) STORED,
    -- Where the archive is in storage, and how big it is.
    `package_key` varchar(255) NOT NULL DEFAULT '',
    `size_bytes` bigint unsigned NOT NULL DEFAULT 0,
    -- Why it failed, in a word.
    `error` varchar(64) NOT NULL DEFAULT '',
    -- How many times a worker has taken it, and when one last did.
    `attempts` int NOT NULL DEFAULT 0,
    `claimed_at` datetime DEFAULT NULL,
    -- Where it was asked for from, kept as the record of the request.
    `ip` varchar(64) NOT NULL DEFAULT '',
    `user_agent` varchar(255) NOT NULL DEFAULT '',
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    `ready_at` datetime DEFAULT NULL,
    -- When the archive is deleted.
    `expires_at` datetime DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_data_exports_active_person` (`active_person_id`),
    KEY `idx_data_exports_person` (`person_id`, `created_at`),
    KEY `idx_data_exports_status` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS data_exports;
