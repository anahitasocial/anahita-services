-- ActivityPub: what is known of the people on other servers, beyond what a
-- node holds.
--
--   activitypub_actors   one row for each person of another server who is
--                        kept here as a node. Where to write to them, and
--                        the key that what they send is checked against.
--                        The node has their name and what they say of
--                        themselves; this has what only this service needs.
--
-- Belongs to activitypub-service.

-- +goose Up
CREATE TABLE IF NOT EXISTS activitypub_actors (
    `node_id` bigint unsigned NOT NULL,
    -- Their address: the same as the node's uri.
    `uri` varchar(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `inbox` varchar(512) NOT NULL,
    -- Where what is for anybody on their server is sent once. '' for none.
    `shared_inbox` varchar(512) NOT NULL DEFAULT '',
    -- The name of the key they sign with, and its public half, as PEM.
    `key_id` varchar(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `public_key` text NOT NULL,
    -- Where their avatar is on their own server. '' for none.
    `icon_url` varchar(512) NOT NULL DEFAULT '',
    -- Whether a follow of theirs waits to be approved.
    `locked` tinyint(1) NOT NULL DEFAULT 0,
    -- When they were last read from their server.
    `fetched_at` datetime NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`node_id`),
    UNIQUE KEY `uq_uri` (`uri`),
    KEY `key_id` (`key_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS activitypub_actors;
