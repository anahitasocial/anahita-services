-- Events take no handle.
--
-- nodes.unique_alias is the one namespace every actor's handle is in, because
-- a handle is how an actor is found from elsewhere and that has no kind in
-- it. An event is an actor, so it was in it too, and should not be:
--
--   - two events could not be called the same thing. The second "Birthday"
--     on an installation could not be made;
--   - an event named for a town took that handle from the group or person
--     who would want it.
--
-- An event is not addressed by a handle. Its address carries its id
-- (/events/42-birthday), and the name after it is for reading.
--
-- So the generated column gives an event NULL, like everything that is not
-- an actor, and a unique index permits any number of those.
--
-- GUARDED: schema/init.sql already has the new expression, and
-- entrypoint.sh runs it on an empty database before stamping the versions.
-- The column is rewritten only where its expression does not mention events.

-- +goose Up
-- The words in the expression are stored with the character set of whoever
-- wrote them. Said here, so the column comes out the same whatever the
-- connection running this happens to use, and the same as schema/init.sql
-- gives a new database.
SET NAMES utf8mb4;
-- +goose StatementBegin
CREATE PROCEDURE anahita_events_take_no_handle()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'unique_alias'
          AND GENERATION_EXPRESSION LIKE '%actor-service.event.v1%'
    ) THEN
        ALTER TABLE nodes
            MODIFY COLUMN `unique_alias` varchar(300)
                GENERATED ALWAYS AS (
                    CASE
                        WHEN alias IS NULL OR alias = '' THEN NULL
                        WHEN type = 'node.actor.actor-service.event.v1' THEN NULL
                        WHEN type LIKE 'node.actor.%'    THEN CONCAT('actor:', alias)
                        ELSE NULL
                    END
                ) STORED;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_events_take_no_handle();
DROP PROCEDURE anahita_events_take_no_handle;

-- +goose Down
-- Not put back. Events made since may share a name, and the old expression
-- would refuse the column for them.
SELECT 1;
