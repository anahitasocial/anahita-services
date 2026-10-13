-- AT Protocol: what is kept for reading Bluesky.
--
--   atproto_actors   one row for each person on Bluesky who is kept here
--                    as a node. The node has their name, their handle and
--                    what they say of themselves; this has what only this
--                    service needs: their lasting name (their DID), where
--                    their pictures were, and whether what they posted
--                    before they were followed has been read.
--   atproto_cursor   one row: how far the listener has got in what
--                    Bluesky has said, so that one that begins again
--                    hears what was posted while nobody listened.
--
-- A follow of somebody on Bluesky is not kept here: it is an edge in the
-- graph, like any follow. Nothing is written to Bluesky yet, so there is
-- no record of what it was told.
--
-- Belongs to atproto-service.

-- +goose Up
CREATE TABLE IF NOT EXISTS atproto_actors (
    `node_id` bigint unsigned NOT NULL,
    -- Their lasting name: what does not change when their handle does.
    `did` varchar(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- The handle they went by when last read.
    `handle` varchar(255) NOT NULL,
    -- Where their avatar and their banner were when last read. '' for none.
    `avatar_url` varchar(512) NOT NULL DEFAULT '',
    `banner_url` varchar(512) NOT NULL DEFAULT '',
    -- When what they posted before they were followed was read. NULL when
    -- it has not been.
    `backfilled_at` datetime DEFAULT NULL,
    -- When they were last read from Bluesky.
    `fetched_at` datetime NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`node_id`),
    UNIQUE KEY `uq_did` (`did`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS atproto_cursor (
    -- Always 1: there is one listener, and one place it has got to.
    `id` tinyint unsigned NOT NULL,
    -- The time of the last thing dealt with, in microseconds.
    `time_us` bigint unsigned NOT NULL,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS atproto_cursor;
DROP TABLE IF EXISTS atproto_actors;
