-- ActivityPub: where the cover of somebody on another server is, there.
--
--   activitypub_actors.header_url   the address of the wide picture across
--                                   the top of their profile, on their own
--                                   server. Kept to tell when it changes: the
--                                   picture itself is copied into this
--                                   installation's storage, as their avatar
--                                   is. '' for none.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_actor_header_url()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'activitypub_actors' AND COLUMN_NAME = 'header_url'
    ) THEN
        ALTER TABLE activitypub_actors ADD COLUMN `header_url` varchar(512) NOT NULL DEFAULT '' AFTER `icon_url`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_actor_header_url();
DROP PROCEDURE anahita_add_actor_header_url;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_actor_header_url()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'activitypub_actors' AND COLUMN_NAME = 'header_url'
    ) THEN
        ALTER TABLE activitypub_actors DROP COLUMN `header_url`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_actor_header_url();
DROP PROCEDURE anahita_drop_actor_header_url;
