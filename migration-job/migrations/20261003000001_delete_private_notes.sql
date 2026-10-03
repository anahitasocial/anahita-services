-- Deletes legacy private notes, and everything that hangs off them.
--
-- In the PHP app a person could leave a private note on somebody else's
-- profile, seen only by the two of them, and carry on in its comments. It was
-- stored as an ordinary note whose `access` held the two person ids
-- ("21010,1") where every other row holds a keyword such as `public`.
--
-- The Go services never had the feature and never will: private messaging is
-- more responsibility than this project wants, and it has no workable
-- equivalent on ActivityPub or AT Protocol. Nothing can create these rows any
-- more (the note validator accepts keywords only), and since the feed gate was
-- fixed nobody can list them, the two participants included. They are data
-- nobody can reach, about conversations people believed were private.
--
-- What goes, counted on the reference database:
--
--     348 notes whose access is a list of ids
--     604 comments under them
--     707 edges touching either (subscriptions, likes, hashtags, mentions)
--     140 notifications about either
--
-- No other node type carries an id list in `access`, and none of these notes
-- was reposted or has a file.
--
-- ORDER. Nothing in this schema has a foreign key, so nothing cascades:
-- notifications and edges go first, while the ids that identify them still
-- exist, then the nodes.
--
-- The ids are collected into temporary tables once, so every step works from
-- the same set. Two tables rather than one, because MySQL cannot read a
-- temporary table twice in one statement.
--
-- IDEMPOTENT: it matches the shape, so a second run finds nothing. The same
-- deletion is in legacy/upgrade.sql, because an install upgraded from the PHP
-- app is stamped as migrated without running this file.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_delete_private_notes()
BEGIN
    DROP TEMPORARY TABLE IF EXISTS anahita_private_notes;
    DROP TEMPORARY TABLE IF EXISTS anahita_private_nodes;

    CREATE TEMPORARY TABLE anahita_private_notes (id BIGINT UNSIGNED NOT NULL PRIMARY KEY);
    CREATE TEMPORARY TABLE anahita_private_nodes (id BIGINT UNSIGNED NOT NULL PRIMARY KEY);

    INSERT INTO anahita_private_notes (id)
    SELECT id FROM nodes
     WHERE type = 'node.medium.text-service.note.v1'
       AND access REGEXP '^[0-9]+(,[0-9]+)*$';

    -- The notes, and whatever sits under them. Comments are flat, so one
    -- level is the whole tree.
    INSERT INTO anahita_private_nodes (id)
    SELECT id FROM anahita_private_notes;

    INSERT IGNORE INTO anahita_private_nodes (id)
    SELECT n.id FROM nodes n
      JOIN anahita_private_notes p ON p.id = n.parent_id;

    DELETE n FROM notifications n JOIN anahita_private_nodes p ON p.id = n.object_id;
    DELETE n FROM notifications n JOIN anahita_private_nodes p ON p.id = n.comment_id;
    DELETE n FROM notifications n JOIN anahita_private_nodes p ON p.id = n.target_id;

    DELETE e FROM edges e JOIN anahita_private_nodes p ON p.id = e.node_a_id;
    DELETE e FROM edges e JOIN anahita_private_nodes p ON p.id = e.node_b_id;

    DELETE n FROM nodes n JOIN anahita_private_nodes p ON p.id = n.id;

    DROP TEMPORARY TABLE anahita_private_notes;
    DROP TEMPORARY TABLE anahita_private_nodes;
END;
-- +goose StatementEnd
CALL anahita_delete_private_notes();
DROP PROCEDURE anahita_delete_private_notes;

-- +goose Down
-- +goose StatementBegin
-- Not reversible, and deliberately so. The rows are gone, and there is nothing
-- to put back that anybody could read. The Down is a no-op rather than a lie.
SELECT 1;
-- +goose StatementEnd
