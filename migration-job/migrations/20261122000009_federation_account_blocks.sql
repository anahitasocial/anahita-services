-- Accounts on other networks refused one by one (anahita-web roadmap row
-- 116, docs/plans/done/refused-accounts.md): kept by the actor's address or the
-- DID, with how they were shown, since what is here of them is removed.

-- +goose Up
CREATE TABLE IF NOT EXISTS federation_account_blocks (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- 'activitypub' or 'atproto'.
    `network` varchar(16) NOT NULL,
    -- The actor's address on the fediverse, or the DID on Bluesky.
    `identifier` varchar(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- How they were shown when refused, for the list.
    `handle` varchar(255) NOT NULL DEFAULT '',
    `name` varchar(255) NOT NULL DEFAULT '',
    -- Why, for the administrators who come after. Never shown to anybody
    -- else, and never sent to the account refused.
    `reason` varchar(500) NOT NULL DEFAULT '',
    `created_by` bigint unsigned NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uq_network_identifier` (`network`, `identifier`),
    KEY `created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS federation_account_blocks;
