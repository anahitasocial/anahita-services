-- Deletes legacy photo sets (albums), and everything that hangs off them.
--
-- The PHP app let people group photos into sets. The upgrade carried the sets
-- over as `node.medium.photo-service.photo-set.v1` nodes, tied to their photos
-- by `edge.photo.photo-set.v1` edges, but no Go service reads, writes or serves
-- either, and the web app has no page for them. They only surfaced as rows in
-- feeds that nothing could open.
--
-- Albums are not coming back: a photo post holds up to four images, and
-- collections were decided against (docs/plans/photo-albums.md).
--
-- THE PHOTOS STAY. A photo belongs to its owner through `owner_id`, never to a
-- set, and its `parent_id` is never a set, so nothing here touches one. A
-- set's `filename` is its cover photo's name, not a file of its own, so there
-- is nothing to remove from storage either.
--
-- What goes, counted on the reference database:
--
--     151 photo sets
--      52 comments under them
--   1,259 edges touching either: 1,018 set memberships, the rest
--         subscriptions, likes, hashtags, locations and mentions
--       1 notification
--
-- ORDER. Nothing in this schema has a foreign key, so nothing cascades:
-- notifications and edges go first, while the ids that identify them still
-- exist, then the nodes. Two temporary tables, because MySQL cannot read one
-- twice in a statement.
--
-- IDEMPOTENT: it matches the type, so a second run finds nothing. The same
-- deletion is in legacy/upgrade.sql, because an install upgraded from the PHP
-- app is stamped as migrated without running this file.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_delete_photo_sets()
BEGIN
    DROP TEMPORARY TABLE IF EXISTS anahita_photo_sets;
    DROP TEMPORARY TABLE IF EXISTS anahita_photo_set_nodes;

    CREATE TEMPORARY TABLE anahita_photo_sets (id BIGINT UNSIGNED NOT NULL PRIMARY KEY);
    CREATE TEMPORARY TABLE anahita_photo_set_nodes (id BIGINT UNSIGNED NOT NULL PRIMARY KEY);

    INSERT INTO anahita_photo_sets (id)
    SELECT id FROM nodes
     WHERE type = 'node.medium.photo-service.photo-set.v1';

    -- The sets, and the comments under them. Only comments: a photo is never
    -- the child of a set, and the type check keeps it that way even if some
    -- row somewhere says otherwise.
    INSERT INTO anahita_photo_set_nodes (id)
    SELECT id FROM anahita_photo_sets;

    INSERT IGNORE INTO anahita_photo_set_nodes (id)
    SELECT n.id FROM nodes n
      JOIN anahita_photo_sets s ON s.id = n.parent_id
     WHERE n.type = 'node.base.comment-service.comment.v1';

    DELETE n FROM notifications n JOIN anahita_photo_set_nodes p ON p.id = n.object_id;
    DELETE n FROM notifications n JOIN anahita_photo_set_nodes p ON p.id = n.comment_id;
    DELETE n FROM notifications n JOIN anahita_photo_set_nodes p ON p.id = n.target_id;

    DELETE e FROM edges e JOIN anahita_photo_set_nodes p ON p.id = e.node_a_id;
    DELETE e FROM edges e JOIN anahita_photo_set_nodes p ON p.id = e.node_b_id;

    -- Membership edges whose set is already gone.
    DELETE FROM edges WHERE type = 'edge.photo.photo-set.v1';

    DELETE n FROM nodes n JOIN anahita_photo_set_nodes p ON p.id = n.id;

    DROP TEMPORARY TABLE anahita_photo_sets;
    DROP TEMPORARY TABLE anahita_photo_set_nodes;
END;
-- +goose StatementEnd
CALL anahita_delete_photo_sets();
DROP PROCEDURE anahita_delete_photo_sets;

-- +goose Down
-- +goose StatementBegin
-- Not reversible, and deliberately so. The rows are gone and nothing could
-- show them. The Down is a no-op rather than a lie.
SELECT 1;
-- +goose StatementEnd
