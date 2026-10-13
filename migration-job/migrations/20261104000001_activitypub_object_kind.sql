-- ActivityPub: what kind of thing was sent.
--
--   activitypub_objects.kind   'note' for a post or a reply, 'repost' for a
--                              repost. Every row from before this is a
--                              note.
--
-- A repost goes out as a thing done (an Announce), and is taken back by
-- undoing it; a note is taken back by saying it is gone. When either is
-- taken back the node itself may be gone already, so which it was is kept
-- with the record that it went.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_object_kind()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'activitypub_objects' AND COLUMN_NAME = 'kind'
    ) THEN
        ALTER TABLE activitypub_objects ADD COLUMN `kind` varchar(16) NOT NULL DEFAULT 'note' AFTER `actor_id`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_object_kind();
DROP PROCEDURE anahita_add_object_kind;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_object_kind()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'activitypub_objects' AND COLUMN_NAME = 'kind'
    ) THEN
        ALTER TABLE activitypub_objects DROP COLUMN `kind`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_object_kind();
DROP PROCEDURE anahita_drop_object_kind;
