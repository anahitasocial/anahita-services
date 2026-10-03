-- Drops the nodes columns nothing uses, and renames `hits` to `view_count`.
--
-- These came over from the PHP schema and were carried through every read and
-- write as plumbing, but no endpoint sets them and no client shows them:
--
--   status, status_update_time   a profile status line. The legacy upgrade
--                                already blanked every one.
--   is_default, ordering         never read. (edges.ordering is a different
--                                column and stays.)
--   timezone                     an integer offset, which cannot follow
--                                daylight saving. Events will store a zone name.
--   last_comment_id, last_comment_by, last_comment_on
--                                never written by comment-service.
--   filesize, cover_filesize, cover_mimetype
--                                never written by the upload paths.
--   mutual_count                 never calculated, only reset on purge.
--   vote_down_count              there is no downvote.
--
-- `status` was also a live fault: TombstoneActor set it to NULL and the node
-- scanner read it as a plain string, so a purged actor's row could not be
-- loaded.
--
-- `hits` becomes `view_count`, named like the other counters, and NOT NULL so
-- it can be incremented without a COALESCE. Nothing increments it yet.
--
-- The three indexes are dropped by name first. Dropping a column removes it
-- from a composite index but leaves the index behind on what remains, here a
-- second index on `type` alone.
--
-- GUARDED, for the same reason as the migrations before it: entrypoint.sh runs
-- schema/init.sql on an empty database and stamps the versions afterwards, so a
-- migration must survive being run against a schema that already has its
-- change.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_index(IN idx VARCHAR(64))
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = idx
    ) THEN
        SET @anahita_sql = CONCAT('ALTER TABLE nodes DROP INDEX `', idx, '`');
        PREPARE anahita_stmt FROM @anahita_sql;
        EXECUTE anahita_stmt;
        DEALLOCATE PREPARE anahita_stmt;
    END IF;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_column(IN col VARCHAR(64))
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = col
    ) THEN
        SET @anahita_sql = CONCAT('ALTER TABLE nodes DROP COLUMN `', col, '`');
        PREPARE anahita_stmt FROM @anahita_sql;
        EXECUTE anahita_stmt;
        DEALLOCATE PREPARE anahita_stmt;
    END IF;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE PROCEDURE anahita_rename_nodes_hits()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'hits'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'view_count'
    ) THEN
        UPDATE nodes SET hits = 0 WHERE hits IS NULL;
        ALTER TABLE nodes CHANGE `hits` `view_count` int(11) unsigned NOT NULL DEFAULT 0;
    END IF;
END;
-- +goose StatementEnd

CALL anahita_drop_nodes_index('type_status_update_time');
CALL anahita_drop_nodes_index('type_default');
CALL anahita_drop_nodes_index('last_comment_by');

CALL anahita_drop_nodes_column('status');
CALL anahita_drop_nodes_column('status_update_time');
CALL anahita_drop_nodes_column('is_default');
CALL anahita_drop_nodes_column('ordering');
CALL anahita_drop_nodes_column('timezone');
CALL anahita_drop_nodes_column('last_comment_id');
CALL anahita_drop_nodes_column('last_comment_by');
CALL anahita_drop_nodes_column('last_comment_on');
CALL anahita_drop_nodes_column('filesize');
CALL anahita_drop_nodes_column('cover_filesize');
CALL anahita_drop_nodes_column('cover_mimetype');
CALL anahita_drop_nodes_column('mutual_count');
CALL anahita_drop_nodes_column('vote_down_count');

CALL anahita_rename_nodes_hits();

DROP PROCEDURE anahita_drop_nodes_index;
DROP PROCEDURE anahita_drop_nodes_column;
DROP PROCEDURE anahita_rename_nodes_hits;

-- +goose Down
-- Brings the columns back EMPTY, with the definitions they had. Their data is
-- gone for good, but the previous version of the code selects every one of
-- them, so a rollback without them would leave it unable to read a node.
-- The indexes are not restored: nothing queried through them.
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_column(IN col VARCHAR(64), IN definition VARCHAR(255))
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = col
    ) THEN
        SET @anahita_sql = CONCAT('ALTER TABLE nodes ADD COLUMN `', col, '` ', definition);
        PREPARE anahita_stmt FROM @anahita_sql;
        EXECUTE anahita_stmt;
        DEALLOCATE PREPARE anahita_stmt;
    END IF;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE PROCEDURE anahita_restore_nodes_hits()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'view_count'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'hits'
    ) THEN
        ALTER TABLE nodes CHANGE `view_count` `hits` int(11) unsigned DEFAULT NULL;
    END IF;
END;
-- +goose StatementEnd

CALL anahita_add_nodes_column('status', 'mediumtext');
CALL anahita_add_nodes_column('status_update_time', 'datetime DEFAULT NULL');
CALL anahita_add_nodes_column('is_default', 'tinyint(1) NOT NULL DEFAULT 0');
CALL anahita_add_nodes_column('ordering', 'int(11) DEFAULT 0');
CALL anahita_add_nodes_column('timezone', 'int(11) DEFAULT NULL');
CALL anahita_add_nodes_column('last_comment_id', 'bigint(11) unsigned DEFAULT NULL');
CALL anahita_add_nodes_column('last_comment_by', 'bigint(11) unsigned DEFAULT NULL');
CALL anahita_add_nodes_column('last_comment_on', 'datetime DEFAULT NULL');
CALL anahita_add_nodes_column('filesize', 'int(11) DEFAULT NULL');
CALL anahita_add_nodes_column('cover_filesize', 'int(11) DEFAULT NULL');
CALL anahita_add_nodes_column('cover_mimetype', 'varchar(100) DEFAULT NULL');
CALL anahita_add_nodes_column('mutual_count', 'int(11) unsigned DEFAULT NULL');
CALL anahita_add_nodes_column('vote_down_count', 'int(11) unsigned DEFAULT NULL');

-- The previous code scans `status` as a plain string, so it must not be NULL.
UPDATE nodes SET status = '' WHERE status IS NULL;

CALL anahita_restore_nodes_hits();

DROP PROCEDURE anahita_add_nodes_column;
DROP PROCEDURE anahita_restore_nodes_hits;
