-- ActivityPub: who has chosen to be on the fediverse, and the key each
-- signs with.
--
--   activitypub_accounts   one row for each person who has ever chosen. Only
--                          somebody with a row that says enabled is spoken
--                          of to other servers at all.
--   actor_keys             the key a node signs with. The public half is
--                          told to anybody; the private half is kept
--                          encrypted, by the means auth-service keeps its
--                          secrets (CRYPT_BACKEND). One key in use for each
--                          node: active_node_id is the node while the key
--                          is in use and NULL once it is revoked, and is
--                          unique.
--
-- Both belong to activitypub-service, which is the only thing that reads
-- or writes them.

-- +goose Up
CREATE TABLE IF NOT EXISTS activitypub_accounts (
    `person_id` bigint unsigned NOT NULL,
    `enabled` tinyint(1) NOT NULL DEFAULT 0,
    -- When they last turned it on.
    `enabled_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`person_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS actor_keys (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `node_id` bigint unsigned NOT NULL,
    `algorithm` varchar(16) NOT NULL,
    -- PEM.
    `public_key` text NOT NULL,
    `private_key_encrypted` text NOT NULL,
    `created_at` datetime NOT NULL,
    `revoked_at` datetime DEFAULT NULL,
    `active_node_id` bigint unsigned GENERATED ALWAYS AS (IF(`revoked_at` IS NULL, `node_id`, NULL)) STORED,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uq_active_node_id` (`active_node_id`),
    KEY `node_id` (`node_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS actor_keys;
DROP TABLE IF EXISTS activitypub_accounts;
