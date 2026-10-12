-- ActivityPub: which posts made here other servers have been told of.
--
--   activitypub_objects   one row for each post that was sent out, or could
--                         be fetched. When a post is deleted here its node is
--                         gone by the time anybody hears of it; this is what
--                         still says what it was called between servers, so
--                         that the servers that have it can be told to drop
--                         it. It is also what stops a post that was never
--                         sent from being "deleted" at them.
--
-- Belongs to activitypub-service.

-- +goose Up
CREATE TABLE IF NOT EXISTS activitypub_objects (
    `node_id` bigint unsigned NOT NULL,
    -- The post's UUID: what its address between servers is made of.
    `uuid` char(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- Whose it is: the person here who wrote it.
    `actor_id` bigint unsigned NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`node_id`),
    KEY `actor_id` (`actor_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS activitypub_objects;
