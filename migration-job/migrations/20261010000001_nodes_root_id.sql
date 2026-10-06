-- nodes.root_id: the post at the top of the thread a reply belongs to.
--
-- A reply is a note with a parent. parent_id says what it answers, which may
-- itself be a reply; root_id says which post the whole thread hangs from. It
-- is the same as parent_id for a direct reply, and NULL on anything that is
-- not a reply.
--
-- It exists so that deciding who may see a reply is one join to one row,
-- whatever the reply's depth. A reply is visible to exactly the people who can
-- see its root. Without this column that would mean walking up through the
-- parents in SQL for every row of every listing.
--
-- GUARDED, for the same reason as the migrations before it: entrypoint.sh runs
-- schema/init.sql on an empty database and stamps the versions afterwards, so a
-- migration must survive being run against a schema that already has its
-- change.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_root_id()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'root_id'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `root_id` bigint(11) unsigned DEFAULT NULL AFTER `parent_type`;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'root_id'
    ) THEN
        ALTER TABLE nodes ADD INDEX `root_id` (`root_id`);
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_root_id();
DROP PROCEDURE anahita_add_nodes_root_id;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_root_id()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'root_id'
    ) THEN
        ALTER TABLE nodes DROP INDEX `root_id`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'root_id'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `root_id`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_root_id();
DROP PROCEDURE anahita_drop_nodes_root_id;
