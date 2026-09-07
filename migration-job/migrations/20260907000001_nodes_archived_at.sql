-- Adds nodes.archived_at, the third and final lifecycle state.
--
-- `enabled` was carrying three meanings at once — administrator suspension,
-- deletion, and "unpublished" — which is why a soft-deleted actor is
-- indistinguishable from a deliberately disabled one in the browse filter.
-- Archive must not become the fourth. Each state gets its own column:
--
--   enabled     — the reversible switch, and nothing else
--   archived_at — permanent; the content stays readable forever
--   deleted_at  — the 30-day countdown to erasure
--
-- Archiving is for an actor that is FINISHED rather than regretted: a group
-- whose project ended, or a person who has left the network or died. Nothing is
-- destroyed and the content keeps its URLs; what stops is being pushed at
-- people who did not ask for it.
--
-- Indexed because every content read filters on it, through a join to the
-- owner — the same shape deleted_at is read in.
--
-- GUARDED, for the same reason as the migrations before it: entrypoint.sh runs
-- schema/init.sql on an empty database and stamps the versions afterwards, so a
-- migration must survive being run against a schema that already has its
-- change.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_archived_at()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'archived_at'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `archived_at` datetime DEFAULT NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'archived_at'
    ) THEN
        ALTER TABLE nodes ADD INDEX `archived_at` (`archived_at`);
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_archived_at();
DROP PROCEDURE anahita_add_nodes_archived_at;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_archived_at()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'archived_at'
    ) THEN
        ALTER TABLE nodes DROP INDEX `archived_at`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'archived_at'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `archived_at`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_archived_at();
DROP PROCEDURE anahita_drop_nodes_archived_at;
