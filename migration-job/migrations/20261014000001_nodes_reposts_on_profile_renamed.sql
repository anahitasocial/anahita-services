-- nodes.hide_reposts_on_profile becomes show_reposts_on_profile, and starts
-- off.
--
-- The column was added the day before this as "hide", off: reposts were among
-- a profile's posts unless its owner said otherwise. That was turned round
-- before anything was released. Reposts are in the list of reposts, and among
-- the posts only when the owner asks. The migration that added the column now
-- adds it under its new name; this one is for a database that already ran it
-- under the old one. Whatever was chosen under the old name is not carried
-- over: it answered a different question.
--
-- Nothing to do on a database that never had the old name.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_rename_nodes_reposts_on_profile()
BEGIN
    DECLARE has_old INT DEFAULT 0;
    DECLARE has_new INT DEFAULT 0;

    SELECT COUNT(*) INTO has_old FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'hide_reposts_on_profile';

    SELECT COUNT(*) INTO has_new FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'show_reposts_on_profile';

    IF has_old = 1 AND has_new = 1 THEN
        ALTER TABLE nodes DROP COLUMN `hide_reposts_on_profile`;
    ELSEIF has_old = 1 THEN
        ALTER TABLE nodes DROP COLUMN `hide_reposts_on_profile`;
        ALTER TABLE nodes ADD COLUMN `show_reposts_on_profile` tinyint(1) NOT NULL DEFAULT 0 AFTER `allow_follow_request`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_rename_nodes_reposts_on_profile();
DROP PROCEDURE anahita_rename_nodes_reposts_on_profile;

-- +goose Down
SELECT 1;
