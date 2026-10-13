-- AT Protocol: which post each repost heard from Bluesky was of.
--
--   atproto_reposts   one row for each repost kept. A repost that is taken
--                     back is heard as its name alone, with nothing said
--                     of what it was a repost of, so that is kept here
--                     when it is made, and the row goes when it is taken
--                     back.
--
-- Belongs to atproto-service.

-- +goose Up
CREATE TABLE IF NOT EXISTS atproto_reposts (
    -- Who reposted: the node of somebody on Bluesky.
    `actor_id` bigint unsigned NOT NULL,
    -- The name of the repost among their records.
    `rkey` varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- What it was a repost of: the node of the post.
    `parent_id` bigint unsigned NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`actor_id`, `rkey`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS atproto_reposts;
