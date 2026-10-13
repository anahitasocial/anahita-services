-- AT Protocol: writing to Bluesky as accounts people here connected.
--
--   atproto_accounts   the Bluesky account somebody here connected, one
--                      for each person and each account. What they do
--                      here to things from Bluesky is done there as it.
--   atproto_sent       what was written for them: a reply, a like, a
--                      repost, each by the name it was written under, so
--                      that it can be taken back and answered, and known
--                      for this installation's own when Bluesky tells of
--                      it.
--   atproto_writes     what waits to be written, tried until it is done:
--                      the same queue as what goes to the fediverse
--                      (anahita-libs/federation, TableSQL).
--
-- Belongs to atproto-service.

-- +goose Up
CREATE TABLE IF NOT EXISTS atproto_accounts (
    -- Who here connected it.
    `person_id` bigint unsigned NOT NULL,
    -- The account's lasting name, the handle it went by when connected,
    -- and the server that holds it.
    `did` varchar(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `handle` varchar(255) NOT NULL,
    `pds` varchar(255) NOT NULL,
    -- The token that gets the next short one, encrypted. The password it
    -- was got with is not kept.
    `refresh_token` text NOT NULL,
    -- When the account's server last refused the token. NULL while it
    -- works.
    `broken_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`person_id`),
    UNIQUE KEY `uq_did` (`did`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS atproto_sent (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- 'post' (a reply), 'like' or 'repost'.
    `kind` varchar(16) NOT NULL,
    `person_id` bigint unsigned NOT NULL,
    -- What it is here: the reply's node, the node that was liked, or the
    -- repost's node.
    `local_id` bigint unsigned NOT NULL,
    -- The account it was written as, and the name it was written under.
    `did` varchar(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `rkey` varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uq_kind_person_local` (`kind`, `person_id`, `local_id`),
    KEY `kind_local` (`kind`, `local_id`),
    KEY `did_rkey` (`did`, `rkey`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS atproto_writes (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    -- Said by whoever adds a row, to add it once however often they try:
    -- what is written and under what name. NULL for no such promise.
    dedupe_key varchar(191) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL,
    -- Who here it is sent for.
    actor_id bigint unsigned NOT NULL,
    -- Where it goes: always Bluesky.
    target varchar(512) NOT NULL,
    payload mediumblob NOT NULL,
    -- pending or failed. What was sent is removed. What a worker has taken
    -- is pending still, with next_attempt_at put off: see Claim.
    status varchar(16) NOT NULL DEFAULT 'pending',
    attempts int unsigned NOT NULL DEFAULT 0,
    next_attempt_at datetime NOT NULL,
    -- When a worker last took it.
    claimed_at datetime DEFAULT NULL,
    last_error varchar(500) DEFAULT NULL,
    created_at datetime NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_dedupe_key (dedupe_key),
    KEY status_next_attempt_at (status, next_attempt_at),
    KEY actor_id (actor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS atproto_writes;
DROP TABLE IF EXISTS atproto_sent;
DROP TABLE IF EXISTS atproto_accounts;
