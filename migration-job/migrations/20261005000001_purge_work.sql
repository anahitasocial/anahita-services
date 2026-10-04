-- A purge that can be stopped and picked up again.
--
-- Purging an account used to be one transaction under a three-second timeout.
-- An account with a few thousand posts ran out of time, rolled back, and sat
-- in the sweeper's queue failing the same way every hour.
--
-- Two additions:
--
--   nodes.purged_at  set when an actor has been reduced to a tombstone. The
--                    sweeper's queue is "deleted and not yet purged"; without
--                    this it re-read every tombstone forever, and once a
--                    hundred of them had built up it never reached a new
--                    deletion at all.
--
--   purge_work       what is left to do for a purge in progress: the nodes
--                    still to delete, and the nodes whose counters must be
--                    counted again afterwards. Rows are removed as the work is
--                    done, so an empty set means finished and an interrupted
--                    purge carries on from what remains.
--
-- purge_work is owned by graph-grpc-service. No foreign keys, like the rest
-- of this schema.
--
-- GUARDED, for the same reason as the migrations before it: entrypoint.sh runs
-- schema/init.sql on an empty database and stamps the versions afterwards, so a
-- migration must survive being run against a schema that already has its
-- change.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_purged_at()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'purged_at'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `purged_at` datetime DEFAULT NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'purged_at'
    ) THEN
        ALTER TABLE nodes ADD INDEX `purged_at` (`purged_at`);
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_purged_at();
DROP PROCEDURE anahita_add_nodes_purged_at;

-- An account purged before this column existed is a tombstone already: it is
-- deleted, disabled, and has an empty name, which nothing else produces.
-- Marking those keeps the sweeper from purging them a second time.
UPDATE nodes
SET purged_at = deleted_at
WHERE type LIKE 'node.actor.%'
  AND deleted_at IS NOT NULL
  AND purged_at IS NULL
  AND enabled = 0
  AND name = ''
  AND EXISTS (
    SELECT 1
    FROM node_deletion_logs log
    WHERE log.node_id = nodes.id
      AND log.action IN ('purged', 'hard-deleted')
  );

CREATE TABLE IF NOT EXISTS purge_work (
    -- The actor being purged.
    `actor_id` bigint unsigned NOT NULL,
    -- 'delete' for a node to remove, or the counter to count again on a node
    -- that stays: 'count_follows', 'count_votes', 'count_subscribers',
    -- 'count_comments', 'count_reposts'.
    `task` varchar(24) NOT NULL,
    -- The node the task applies to.
    `node_id` bigint unsigned NOT NULL,
    PRIMARY KEY (`actor_id`, `task`, `node_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS purge_work;

-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_purged_at()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'purged_at'
    ) THEN
        ALTER TABLE nodes DROP INDEX `purged_at`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'purged_at'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `purged_at`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_purged_at();
DROP PROCEDURE anahita_drop_nodes_purged_at;
