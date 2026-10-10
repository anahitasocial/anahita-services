-- An event open to the followers of the group hosting it.
--
-- A group's event for the group's own people had to be made visible to
-- everybody, or its followers invited one at a time. This says, on the
-- event, that whoever follows its host may see that it is on and answer it.
-- It is not a new access level: the event's access stays what it was, and
-- what is posted on it is still for the people who answered.
--
-- GUARDED, like the migrations before it: schema/init.sql has the column
-- already on a new database.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_events_open_to_host_followers()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'events'
          AND COLUMN_NAME = 'open_to_host_followers'
    ) THEN
        ALTER TABLE events
            ADD COLUMN `open_to_host_followers` tinyint(1) NOT NULL DEFAULT 0 AFTER `online_url`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_events_open_to_host_followers();
DROP PROCEDURE anahita_events_open_to_host_followers;

-- +goose Down
ALTER TABLE events DROP COLUMN `open_to_host_followers`;
