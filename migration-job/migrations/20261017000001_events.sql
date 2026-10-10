-- Events: an actor people say they are going to.
--
-- An event is a node like a group: followed, posted on, administered. What
-- only an event has is kept here, one row an event: its time zone, how many
-- may go, where to join online, and whether it was called off. Saying you are
-- going is following it, with the answer in the follow edge's meta.
--
-- Owned by graph-grpc-service. No foreign keys, like the rest of this schema.
--
-- GUARDED by IF NOT EXISTS: entrypoint.sh runs schema/init.sql on an empty
-- database and stamps the versions afterwards, so a migration must survive
-- being run against a schema that already has its change.

-- +goose Up
CREATE TABLE IF NOT EXISTS events (
    -- The event, which is an actor: node.actor.actor-service.event.v1. Its
    -- name, description, access and when it starts and ends are on the node.
    `node_id` bigint unsigned NOT NULL,
    -- The zone the times were given in, as IANA names it (America/Toronto),
    -- so they read correctly on either side of a clock change.
    `timezone_name` varchar(64) NOT NULL DEFAULT '',
    -- How many may be going. 0 for no limit.
    `capacity` int unsigned NOT NULL DEFAULT 0,
    -- Where to join when it is online. Sent only to whoever is going, and to
    -- whoever looks after the event.
    `online_url` varchar(512) NOT NULL DEFAULT '',
    -- Set when it is called off. It is kept, so people can see that it was.
    `cancelled_at` datetime DEFAULT NULL,
    -- Set when whoever is going has been reminded, so it is done once.
    `reminded_at` datetime DEFAULT NULL,
    PRIMARY KEY (`node_id`),
    KEY `idx_events_reminded_at` (`reminded_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS events;
