-- ActivityPub: what is waiting to be sent, and who follows whom across
-- servers.
--
--   activitypub_deliveries   the queue of what is to be sent to other
--                            servers. Its shape is the shared one
--                            (anahita-libs/federation, TableSQL), which a
--                            test holds this to.
--   activitypub_follows      a follow between somebody here and somebody on
--                            another server, either way round. The edge in
--                            the graph says that it is so; this says what
--                            the other server calls it, which is what has to
--                            be quoted back to accept it or to undo it.
--
-- Both belong to activitypub-service.

-- +goose Up
CREATE TABLE IF NOT EXISTS activitypub_deliveries (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    -- Said by whoever adds a row, to add it once however often they try:
    -- the activity and the inbox it is for. NULL for no such promise.
    dedupe_key varchar(191) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL,
    -- Who here it is sent for.
    actor_id bigint unsigned NOT NULL,
    -- Where it goes.
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

CREATE TABLE IF NOT EXISTS activitypub_follows (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- The person here, and the person elsewhere.
    `local_id` bigint unsigned NOT NULL,
    `remote_id` bigint unsigned NOT NULL,
    -- 'in': the person elsewhere follows the one here. 'out': the other way.
    `direction` varchar(3) NOT NULL,
    -- What the follow is called between servers: the address of the Follow
    -- that asked for it.
    `activity_id` varchar(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- 'pending' until it is accepted, then 'accepted'.
    `state` varchar(16) NOT NULL DEFAULT 'pending',
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uq_pair` (`local_id`, `remote_id`, `direction`),
    KEY `remote_id` (`remote_id`),
    KEY `activity_id` (`activity_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS activitypub_follows;
DROP TABLE IF EXISTS activitypub_deliveries;
