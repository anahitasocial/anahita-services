-- ActivityPub: where a reply was sent, beyond its author's followers.
--
--   activitypub_objects.reply_inbox   the inbox of whoever the reply
--                                     answers, when they are on another
--                                     server. '' for a post that answers
--                                     nobody elsewhere.
--
-- A reply goes to its author's followers and to whoever it answers. When it
-- is taken away, both have to be told, and by then the reply itself may be
-- gone: where it went is kept with the record that it went.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_object_reply_inbox()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'activitypub_objects' AND COLUMN_NAME = 'reply_inbox'
    ) THEN
        ALTER TABLE activitypub_objects ADD COLUMN `reply_inbox` varchar(512) NOT NULL DEFAULT '' AFTER `actor_id`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_object_reply_inbox();
DROP PROCEDURE anahita_add_object_reply_inbox;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_object_reply_inbox()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'activitypub_objects' AND COLUMN_NAME = 'reply_inbox'
    ) THEN
        ALTER TABLE activitypub_objects DROP COLUMN `reply_inbox`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_object_reply_inbox();
DROP PROCEDURE anahita_drop_object_reply_inbox;
