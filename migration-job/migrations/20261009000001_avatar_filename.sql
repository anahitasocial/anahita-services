-- An actor's avatar gets a column of its own, nodes.avatar_filename.
--
-- Until now it was kept in nodes.filename, which on a post names the file the
-- post is: a photo post's photo. One column meant two things, and which one
-- depended on the kind of node. Code that read or cleared an avatar had to
-- check that the node was an actor first, and code that forgot was wrong
-- quietly.
--
-- From here `filename` means the file a node is, and nothing else. An avatar
-- is in `avatar_filename`, beside `cover_filename`.
--
-- Nothing is moved in storage. The stored name is the same; only the column
-- that holds it changes.
--
-- GUARDED, for the same reason as the migrations before it: entrypoint.sh runs
-- schema/init.sql on an empty database and stamps the versions afterwards, so a
-- migration must survive being run against a schema that already has its
-- change.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_avatar_filename()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'avatar_filename'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `avatar_filename` varchar(255) DEFAULT NULL AFTER `filename`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_avatar_filename();
DROP PROCEDURE anahita_add_nodes_avatar_filename;

-- An actor's avatar moves to the column that is now its own. `filename` is
-- cleared on actors, so it means one thing on every row that has it.
--
-- Running it again changes nothing: afterwards no actor has a filename to
-- move.
UPDATE nodes
SET avatar_filename = filename,
    filename = NULL
WHERE type LIKE 'node.actor.%'
  AND filename IS NOT NULL
  AND filename <> ''
  AND (avatar_filename IS NULL OR avatar_filename = '');

-- +goose Down
-- The avatars go back to where they were before the column is dropped.
UPDATE nodes
SET filename = avatar_filename
WHERE type LIKE 'node.actor.%'
  AND avatar_filename IS NOT NULL
  AND avatar_filename <> '';

-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_avatar_filename()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'avatar_filename'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `avatar_filename`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_avatar_filename();
DROP PROCEDURE anahita_drop_nodes_avatar_filename;
