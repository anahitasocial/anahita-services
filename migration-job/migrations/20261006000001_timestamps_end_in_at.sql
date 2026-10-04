-- One name for timestamps: a date with a time ends in `_at`.
--
-- The schema carried three conventions. The tables that came from the PHP
-- application used `_on` and `_date` (created_on, modified_on, start_date,
-- last_visit_date), tokens had a `start_time`, and everything written since
-- uses `_at`. A reader had to remember which table was which. Thirty-three
-- columns already end in `_at`; these fourteen are the rest.
--
--   created_on       -> created_at
--   modified_on      -> updated_at   (what every newer table already calls it)
--   last_visit_date  -> last_visited_at
--   start_date       -> starts_at
--   end_date         -> ends_at
--   start_time       -> started_at
--
-- `_on` is kept for a calendar date with no time, a DATE column. There are
-- none today.
--
-- A rename changes only the table's definition, so it is immediate whatever
-- the size of the table. An index follows its column; the indexes NAMED after
-- an old column are renamed as well, so the name still says what it covers.
-- One of them, type_modifed_on, loses a spelling mistake on the way.
--
-- GUARDED, for the same reason as the migrations before it: entrypoint.sh runs
-- schema/init.sql on an empty database and stamps the versions afterwards, so
-- a migration must survive being run against a schema that already has its
-- change. Each rename happens only if the old name is there and the new one
-- is not.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_rename_column(IN tbl VARCHAR(64), IN old_name VARCHAR(64), IN new_name VARCHAR(64))
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = tbl
          AND COLUMN_NAME = old_name
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = tbl
          AND COLUMN_NAME = new_name
    ) THEN
        SET @anahita_rename = CONCAT(
            'ALTER TABLE `', tbl, '` RENAME COLUMN `', old_name, '` TO `', new_name, '`');
        PREPARE stmt FROM @anahita_rename;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE PROCEDURE anahita_rename_index(IN tbl VARCHAR(64), IN old_name VARCHAR(64), IN new_name VARCHAR(64))
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = tbl
          AND INDEX_NAME = old_name
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = tbl
          AND INDEX_NAME = new_name
    ) THEN
        SET @anahita_rename = CONCAT(
            'ALTER TABLE `', tbl, '` RENAME INDEX `', old_name, '` TO `', new_name, '`');
        PREPARE stmt FROM @anahita_rename;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;
END;
-- +goose StatementEnd

CALL anahita_rename_column('nodes', 'created_on', 'created_at');
CALL anahita_rename_column('nodes', 'modified_on', 'updated_at');
CALL anahita_rename_column('nodes', 'last_visit_date', 'last_visited_at');
CALL anahita_rename_column('nodes', 'start_date', 'starts_at');
CALL anahita_rename_column('nodes', 'end_date', 'ends_at');
CALL anahita_rename_column('edges', 'created_on', 'created_at');
CALL anahita_rename_column('edges', 'modified_on', 'updated_at');
CALL anahita_rename_column('edges', 'start_date', 'starts_at');
CALL anahita_rename_column('edges', 'end_date', 'ends_at');
CALL anahita_rename_column('notifications', 'created_on', 'created_at');
CALL anahita_rename_column('notifications', 'modified_on', 'updated_at');
CALL anahita_rename_column('auth_logs', 'created_on', 'created_at');
CALL anahita_rename_column('node_deletion_logs', 'created_on', 'created_at');
CALL anahita_rename_column('tokens', 'start_time', 'started_at');

CALL anahita_rename_index('nodes', 'start_date', 'starts_at');
CALL anahita_rename_index('nodes', 'end_date', 'ends_at');
CALL anahita_rename_index('nodes', 'type_modifed_on', 'type_updated_at');
CALL anahita_rename_index('nodes', 'type_created_on', 'type_created_at');
CALL anahita_rename_index('nodes', 'idx_nodes_created_on', 'idx_nodes_created_at');
CALL anahita_rename_index('edges', 'start_date', 'starts_at');
CALL anahita_rename_index('edges', 'end_date', 'ends_at');
CALL anahita_rename_index('notifications', 'created_on', 'created_at');
CALL anahita_rename_index('notifications', 'modified_on', 'updated_at');
CALL anahita_rename_index('auth_logs', 'idx_auth_logs_created_on', 'idx_auth_logs_created_at');
CALL anahita_rename_index('auth_logs', 'idx_auth_logs_created_by_client_id_created_on', 'idx_auth_logs_created_by_client_id_created_at');
CALL anahita_rename_index('node_deletion_logs', 'idx_node_deletion_logs_created_on', 'idx_node_deletion_logs_created_at');

DROP PROCEDURE anahita_rename_column;
DROP PROCEDURE anahita_rename_index;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_rename_column(IN tbl VARCHAR(64), IN old_name VARCHAR(64), IN new_name VARCHAR(64))
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = tbl
          AND COLUMN_NAME = old_name
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = tbl
          AND COLUMN_NAME = new_name
    ) THEN
        SET @anahita_rename = CONCAT(
            'ALTER TABLE `', tbl, '` RENAME COLUMN `', old_name, '` TO `', new_name, '`');
        PREPARE stmt FROM @anahita_rename;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE PROCEDURE anahita_rename_index(IN tbl VARCHAR(64), IN old_name VARCHAR(64), IN new_name VARCHAR(64))
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = tbl
          AND INDEX_NAME = old_name
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = tbl
          AND INDEX_NAME = new_name
    ) THEN
        SET @anahita_rename = CONCAT(
            'ALTER TABLE `', tbl, '` RENAME INDEX `', old_name, '` TO `', new_name, '`');
        PREPARE stmt FROM @anahita_rename;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;
END;
-- +goose StatementEnd

CALL anahita_rename_index('nodes', 'starts_at', 'start_date');
CALL anahita_rename_index('nodes', 'ends_at', 'end_date');
CALL anahita_rename_index('nodes', 'type_updated_at', 'type_modifed_on');
CALL anahita_rename_index('nodes', 'type_created_at', 'type_created_on');
CALL anahita_rename_index('nodes', 'idx_nodes_created_at', 'idx_nodes_created_on');
CALL anahita_rename_index('edges', 'starts_at', 'start_date');
CALL anahita_rename_index('edges', 'ends_at', 'end_date');
CALL anahita_rename_index('notifications', 'created_at', 'created_on');
CALL anahita_rename_index('notifications', 'updated_at', 'modified_on');
CALL anahita_rename_index('auth_logs', 'idx_auth_logs_created_at', 'idx_auth_logs_created_on');
CALL anahita_rename_index('auth_logs', 'idx_auth_logs_created_by_client_id_created_at', 'idx_auth_logs_created_by_client_id_created_on');
CALL anahita_rename_index('node_deletion_logs', 'idx_node_deletion_logs_created_at', 'idx_node_deletion_logs_created_on');

CALL anahita_rename_column('nodes', 'created_at', 'created_on');
CALL anahita_rename_column('nodes', 'updated_at', 'modified_on');
CALL anahita_rename_column('nodes', 'last_visited_at', 'last_visit_date');
CALL anahita_rename_column('nodes', 'starts_at', 'start_date');
CALL anahita_rename_column('nodes', 'ends_at', 'end_date');
CALL anahita_rename_column('edges', 'created_at', 'created_on');
CALL anahita_rename_column('edges', 'updated_at', 'modified_on');
CALL anahita_rename_column('edges', 'starts_at', 'start_date');
CALL anahita_rename_column('edges', 'ends_at', 'end_date');
CALL anahita_rename_column('notifications', 'created_at', 'created_on');
CALL anahita_rename_column('notifications', 'updated_at', 'modified_on');
CALL anahita_rename_column('auth_logs', 'created_at', 'created_on');
CALL anahita_rename_column('node_deletion_logs', 'created_at', 'created_on');
CALL anahita_rename_column('tokens', 'started_at', 'start_time');

DROP PROCEDURE anahita_rename_column;
DROP PROCEDURE anahita_rename_index;
