-- nodes.reply_access: who may reply to a post, as its author sets it.
--
-- NULL when the author has not said, which means whoever the profile lets
-- reply. Otherwise 'anyone', 'nobody', or any of 'followers', 'following'
-- and 'mentioned' joined by commas; see anahita-libs/constants/reply_access.go.
--
-- It sits beside comment_status, the older on-and-off switch, and the two
-- are kept in step: 'nobody' is the switch off. A post whose switch was off
-- before this column existed is still closed, and is read that way without
-- this column being filled in for it.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_reply_access()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'reply_access'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `reply_access` varchar(64) DEFAULT NULL AFTER `comment_status`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_reply_access();
DROP PROCEDURE anahita_add_nodes_reply_access;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_reply_access()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'reply_access'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `reply_access`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_reply_access();
DROP PROCEDURE anahita_drop_nodes_reply_access;
