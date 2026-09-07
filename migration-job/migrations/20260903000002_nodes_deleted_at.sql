-- Adds nodes.deleted_at, the flag that turns profile deletion from a hard
-- DELETE into something recoverable.
--
-- Deletion sets this and clears `enabled`. The account stops resolving at every
-- login path straight away, the content graph is left intact, and a sweeper
-- purges after the grace period — so a misclick is a bad afternoon rather than
-- an unrecoverable one. Before this there was no soft delete anywhere in the
-- schema and no way back.
--
-- GUARDED, for the same reason as 20260903000001: entrypoint.sh runs
-- schema/init.sql on an empty database and exits WITHOUT recording anything in
-- goose_db_version, so the next start of that same database finds tables, finds
-- no legacy tables, and runs every migration here from zero against a schema
-- that already has them. A plain ALTER would fail on the second container start
-- of every freshly initialized deployment.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_deleted_at()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'deleted_at'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `deleted_at` datetime DEFAULT NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'deleted_at'
    ) THEN
        ALTER TABLE nodes ADD INDEX `deleted_at` (`deleted_at`);
    END IF;
END
-- +goose StatementEnd
CALL anahita_add_nodes_deleted_at();
DROP PROCEDURE anahita_add_nodes_deleted_at;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_deleted_at()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'deleted_at'
    ) THEN
        ALTER TABLE nodes DROP INDEX `deleted_at`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'deleted_at'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `deleted_at`;
    END IF;
END
-- +goose StatementEnd
CALL anahita_drop_nodes_deleted_at();
DROP PROCEDURE anahita_drop_nodes_deleted_at;
