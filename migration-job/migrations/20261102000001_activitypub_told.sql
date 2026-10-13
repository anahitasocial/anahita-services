-- ActivityPub: what other servers were last told of a post and of a person.
--
--   activitypub_objects.told_updated_at    the post's own updated_at when
--                                          other servers were last sent it.
--   activitypub_accounts.told_updated_at   the person's own updated_at when
--                                          their followers' servers were
--                                          last sent their description.
--
-- A change is told to other servers when this installation hears of it on
-- its bus, which can lose a message. With these, the sweep for what the
-- bus lost can see a change nobody was told of: one made later than what
-- is recorded here. NULL for never recorded, which is every row from
-- before this; such a post is compared with when it was made, such a
-- person with when they joined the fediverse.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_activitypub_told()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'activitypub_objects' AND COLUMN_NAME = 'told_updated_at'
    ) THEN
        ALTER TABLE activitypub_objects ADD COLUMN `told_updated_at` datetime DEFAULT NULL AFTER `actor_id`;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'activitypub_accounts' AND COLUMN_NAME = 'told_updated_at'
    ) THEN
        ALTER TABLE activitypub_accounts ADD COLUMN `told_updated_at` datetime DEFAULT NULL AFTER `enabled_at`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_activitypub_told();
DROP PROCEDURE anahita_add_activitypub_told;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_activitypub_told()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'activitypub_objects' AND COLUMN_NAME = 'told_updated_at'
    ) THEN
        ALTER TABLE activitypub_objects DROP COLUMN `told_updated_at`;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'activitypub_accounts' AND COLUMN_NAME = 'told_updated_at'
    ) THEN
        ALTER TABLE activitypub_accounts DROP COLUMN `told_updated_at`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_activitypub_told();
DROP PROCEDURE anahita_drop_activitypub_told;
