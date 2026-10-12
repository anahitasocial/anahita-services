-- A UUID on every node.
--
-- A node's number says how many there were before it and is only this
-- installation's; its alias can be changed. What other servers are told a
-- person or a post is called has to be neither: once sent it is kept, by
-- servers this one will never hear from again. So every node has a UUID,
-- random (version 4), and that is what its address is made of when it is
-- spoken of to another server.
--
--   nodes.uuid   given by the database when a row is added, so that nothing
--                which adds one can forget to. Unique.
--
-- IN THREE STEPS. A default that is different each time cannot be given to
-- the rows already there by adding the column with it (error 1674: the
-- statement would not repeat on a replica). So the column is added empty,
-- the rows there are filled, and then it is made required with its default.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_node_uuid()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'uuid'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `uuid` char(36) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER `id`;
    END IF;

    UPDATE nodes
    SET `uuid` = (LOWER(CONCAT(HEX(RANDOM_BYTES(4)),'-',HEX(RANDOM_BYTES(2)),'-4',SUBSTR(HEX(RANDOM_BYTES(2)),2),'-',HEX(CONV(SUBSTR(HEX(RANDOM_BYTES(1)),1,1),16,10) & 3 | 8),SUBSTR(HEX(RANDOM_BYTES(2)),2),'-',HEX(RANDOM_BYTES(6)))))
    WHERE `uuid` IS NULL;

    ALTER TABLE nodes MODIFY COLUMN `uuid` char(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT (LOWER(CONCAT(HEX(RANDOM_BYTES(4)),'-',HEX(RANDOM_BYTES(2)),'-4',SUBSTR(HEX(RANDOM_BYTES(2)),2),'-',HEX(CONV(SUBSTR(HEX(RANDOM_BYTES(1)),1,1),16,10) & 3 | 8),SUBSTR(HEX(RANDOM_BYTES(2)),2),'-',HEX(RANDOM_BYTES(6)))));

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND INDEX_NAME = 'uq_uuid'
    ) THEN
        ALTER TABLE nodes ADD UNIQUE KEY `uq_uuid` (`uuid`);
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_node_uuid();
DROP PROCEDURE anahita_add_node_uuid;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_node_uuid()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'uuid'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `uuid`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_node_uuid();
DROP PROCEDURE anahita_drop_node_uuid;
