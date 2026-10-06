-- nodes.show_reposts_on_profile: whether what a person or a group reposts is
-- also shown among the posts on their profile.
--
-- Off for everybody until they say otherwise. A profile's posts are what was
-- posted on it, and what its owner reposted is in the list of reposts beside
-- them. On, reposts are among the posts as well. Followers are sent reposts
-- in their own feeds either way; that is what a repost is for.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_show_reposts_on_profile()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME IN ('show_reposts_on_profile', 'hide_reposts_on_profile')
    ) THEN
        ALTER TABLE nodes ADD COLUMN `show_reposts_on_profile` tinyint(1) NOT NULL DEFAULT 0 AFTER `allow_follow_request`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_show_reposts_on_profile();
DROP PROCEDURE anahita_add_nodes_show_reposts_on_profile;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_show_reposts_on_profile()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'show_reposts_on_profile'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `show_reposts_on_profile`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_show_reposts_on_profile();
DROP PROCEDURE anahita_drop_nodes_show_reposts_on_profile;
