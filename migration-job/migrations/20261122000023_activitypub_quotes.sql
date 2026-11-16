-- +goose Up
-- Quotes between servers (roadmap row 156, FEP-044f). A quote here of a
-- post on another server asks that post's author to let it, and keeps
-- what they answered: the address of their stamp.
CREATE TABLE IF NOT EXISTS activitypub_quote_requests (
    -- The note here that quotes.
    `node_id` bigint unsigned NOT NULL,
    -- The QuoteRequest's own address, which their answer names.
    `request_id` varchar(512) NOT NULL,
    -- The post quoted, and its author, by their addresses there.
    `quoted_uri` varchar(512) NOT NULL,
    `quoted_author_uri` varchar(512) NOT NULL,
    -- 'pending', 'accepted' or 'rejected'.
    `state` varchar(16) NOT NULL DEFAULT 'pending',
    -- The stamp an Accept gave: where it is kept, on their server.
    `stamp_uri` varchar(512) NOT NULL DEFAULT '',
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`node_id`),
    KEY `request_id` (`request_id`(191))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- The stamps this installation gave a post on another server that quotes
-- one here: kept so that each is served at its own address, and goes when
-- the quoted post goes.
CREATE TABLE IF NOT EXISTS activitypub_quote_stamps (
    `uuid` char(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- The post here quoted, and the person here who let it be.
    `quoted_node_id` bigint unsigned NOT NULL,
    `author_id` bigint unsigned NOT NULL,
    -- The quote, and who wrote it, by their addresses there.
    `quote_uri` varchar(512) NOT NULL,
    `quote_author_uri` varchar(512) NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`uuid`),
    UNIQUE KEY `quote` (`quoted_node_id`, `quote_uri`(191))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS activitypub_quote_stamps;
DROP TABLE IF EXISTS activitypub_quote_requests;
