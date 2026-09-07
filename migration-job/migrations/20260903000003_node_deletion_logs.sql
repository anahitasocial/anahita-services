-- Adds node_deletion_logs, the audit trail for deletion of any node.
--
-- Deletion left no record at all: person-service published an event nothing
-- consumed, group-service published nothing, and the only trace was a line in a
-- container's stdout that went with the next restart. "Who removed this member,
-- and when" had no answer.
--
-- Keyed on node_id rather than on an actor: nothing about this record is
-- specific to profiles, and a deleted photo or article wants the same row for
-- the same reason. Actors are simply the first thing to write to it.
--
-- Three actions make up one node's story — scheduled, purged, restored — and the
-- rows outlive the node on purpose. An audit trail erased along with its subject
-- is not an audit trail.
--
-- GUARDED, for the same reason as the migrations before it: entrypoint.sh runs
-- schema/init.sql on an empty database and exits WITHOUT recording anything in
-- goose_db_version, so the next start of that database runs every migration
-- here against a schema that already has them.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_node_deletion_logs()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'node_deletion_logs'
    ) THEN
        CREATE TABLE node_deletion_logs (
            `id` SERIAL,
            `node_id` bigint(11) unsigned NOT NULL,
            `node_alias` varchar(255) DEFAULT NULL,
            `node_type` varchar(255) DEFAULT NULL,
            `deleted_by` bigint(11) unsigned NOT NULL,
            `action` varchar(20) NOT NULL,
            `created_on` datetime NOT NULL,
            PRIMARY KEY (`id`),
            KEY `idx_node_deletion_logs_node_id` (`node_id`),
            KEY `idx_node_deletion_logs_created_on` (`created_on`)
        ) ENGINE=InnoDB CHARACTER SET=utf8;
    END IF;
END
-- +goose StatementEnd
CALL anahita_add_node_deletion_logs();
DROP PROCEDURE anahita_add_node_deletion_logs;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_node_deletion_logs()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'node_deletion_logs'
    ) THEN
        DROP TABLE node_deletion_logs;
    END IF;
END
-- +goose StatementEnd
CALL anahita_drop_node_deletion_logs();
DROP PROCEDURE anahita_drop_node_deletion_logs;
