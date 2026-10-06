-- nodes.hide_reposts_on_profile: whether a person or a group keeps what they
-- repost out of the list of posts on their profile.
--
-- Off for everybody until they say otherwise: a repost shows among a
-- profile's posts, as it does on the networks this will federate with. On,
-- reposts are only in the profile's list of reposts. Followers are sent them
-- in their own feeds either way; that is what a repost is for.
--
-- Stored as "hide" and not as "show" so that a row written without it, and
-- every row from before it, means what was true before it existed.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_hide_reposts_on_profile()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'hide_reposts_on_profile'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `hide_reposts_on_profile` tinyint(1) NOT NULL DEFAULT 0 AFTER `allow_follow_request`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_hide_reposts_on_profile();
DROP PROCEDURE anahita_add_nodes_hide_reposts_on_profile;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_hide_reposts_on_profile()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'hide_reposts_on_profile'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `hide_reposts_on_profile`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_hide_reposts_on_profile();
DROP PROCEDURE anahita_drop_nodes_hide_reposts_on_profile;
