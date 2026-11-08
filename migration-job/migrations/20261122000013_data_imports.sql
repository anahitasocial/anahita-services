-- +goose Up
-- A person's lists brought in from a file (anahita-web roadmap row 124):
-- who they follow, block and mute, and what they saved, from another
-- Anahita's download or from a single CSV, Mastodon's or Misskey's. Owned
-- by import-export-service and its worker. No foreign keys, like the rest
-- of the schema.
CREATE TABLE IF NOT EXISTS data_imports (
    -- Public, and unguessable: it is in the addresses its owner asks.
    `id` char(36) NOT NULL,
    `person_id` bigint unsigned NOT NULL,
    -- 'ready' once the file is read and what is in it is shown, until the
    -- person starts it; 'running' while the worker goes through it; then
    -- 'done', or 'cancelled' for one given up.
    `status` varchar(16) NOT NULL DEFAULT 'ready',
    -- The person, while the import is waiting or running, and NULL
    -- afterwards: one at a time for each person, a rule of the database.
    `active_person_id` bigint unsigned GENERATED ALWAYS AS (IF(`status` IN ('ready', 'running'), `person_id`, NULL)) STORED,
    -- What the file was: 'anahita' (a download), 'mastodon' (a CSV of
    -- Mastodon's or Misskey's), and its name as uploaded.
    `source` varchar(16) NOT NULL DEFAULT '',
    `file_name` varchar(255) NOT NULL DEFAULT '',
    -- When the worker last went through a line of it: whoever takes it after
    -- a restart goes on from there.
    `claimed_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    `started_at` datetime DEFAULT NULL,
    `finished_at` datetime DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_data_imports_active_person` (`active_person_id`),
    KEY `idx_data_imports_person` (`person_id`, `created_at`),
    KEY `idx_data_imports_status` (`status`, `claimed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Each line of an import's file: one person to follow, block or mute, or
-- one post to save, and what came of it.
CREATE TABLE IF NOT EXISTS data_import_lines (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `import_id` char(36) NOT NULL,
    -- 'follow', 'block', 'mute' or 'save'.
    `kind` varchar(16) NOT NULL,
    -- Who or what: a person's handle, a post's ActivityPub id or at://
    -- address, and, from a download, the name and page it had there.
    `address` varchar(512) NOT NULL DEFAULT '',
    `uri` varchar(512) NOT NULL DEFAULT '',
    `name` varchar(255) NOT NULL DEFAULT '',
    `url` varchar(512) NOT NULL DEFAULT '',
    -- For a mute: whether it hides their notifications, and when it ends.
    `hide_notifications` tinyint(1) NOT NULL DEFAULT 1,
    `ends_at` datetime DEFAULT NULL,
    -- 'pending'; 'working' while the worker has it; then 'done', 'already'
    -- (there was nothing to do), 'skipped' (a kind the person did not
    -- choose) or 'failed', with why in a word.
    `status` varchar(16) NOT NULL DEFAULT 'pending',
    `reason` varchar(64) NOT NULL DEFAULT '',
    -- When the worker took it or finished it. One left 'working' for long,
    -- by a worker that stopped, is taken again.
    `touched_at` datetime DEFAULT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_data_import_lines_import` (`import_id`, `status`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS data_import_lines;
DROP TABLE IF EXISTS data_imports;
