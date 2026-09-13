-- Repairs edge data that legacy/upgrade.sql got wrong.
--
-- The same corrections are now in legacy/upgrade.sql, but that file only ever
-- runs against a database that still has prefixed tables. Once an install has
-- been through it, entrypoint.sh only ever reaches the goose migrations again —
-- so every database already migrated keeps these defects permanently unless
-- they are repaired here too.
--
-- All four are idempotent: they match only the broken shapes, so running this
-- against a database upgraded by the fixed upgrade.sql changes nothing.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_repair_legacy_edge_types()
BEGIN
    -- 1. Tag edges never had their node_a_type migrated.
    --
    -- upgrade.sql matched the EDGE's entity name against a column that holds
    -- the NODE's. A hashtag edge is `...entity.tag` while the hashtag node it
    -- points at is `...entity.hashtag`, so the pattern matched nothing and
    -- every tag edge kept its legacy value: 8,314 hashtag and 587 location
    -- edges on the reference database.
    --
    -- node_a_type is a query filter (edges.go, edges_read.go, edges_delete.go)
    -- and part of uc_edge, so a caller passing the modern type matched nothing:
    -- untagging silently did nothing, and re-tagging inserted a second edge
    -- instead of colliding with the first.
    UPDATE edges SET node_a_type = 'node.tag.hashtag-service.hashtag.v1'
     WHERE node_a_type LIKE '%com:hashtags.domain.entity.hashtag%';
    UPDATE edges SET node_a_type = 'node.tag.location-service.location.v1'
     WHERE node_a_type LIKE '%com:locations.domain.entity.location%';

    -- 2. Comment edges point at a type string that does not exist.
    --
    -- upgrade.sql wrote `node.comment.comment-service.comment.v1`, which
    -- appears in no table, in no constant, and nowhere else in the workspace —
    -- the nodes themselves are `node.base.comment-service.comment.v1`
    -- (constants.NodeBaseCommentEntityType). It also covered only four of the
    -- nine components that can be commented on, so the rest kept their legacy
    -- value. 3,294 of 3,295 comment edges disagreed with the node they point
    -- at; the one correct row was written by the Go stack.
    UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1'
     WHERE node_b_type = 'node.comment.comment-service.comment.v1'
        OR node_b_type LIKE '%com:stories.domain.entity.comment%'
        OR node_b_type LIKE '%com:groups.domain.entity.comment%'
        OR node_b_type LIKE '%com:people.domain.entity.comment%'
        OR node_b_type LIKE '%com:photos.domain.entity.comment%'
        OR node_b_type LIKE '%com:topics.domain.entity.comment%'
        OR node_b_type LIKE '%com:notes.domain.entity.comment%'
        OR node_b_type LIKE '%com:pages.domain.entity.comment%'
        OR node_b_type LIKE '%com:articles.domain.entity.comment%'
        OR node_b_type LIKE '%com:todos.domain.entity.comment%';

    -- 3. Nodes the component filter never matched.
    --
    -- upgrade.sql deletes by `component NOT IN (...)`, and these rows carry a
    -- type string where a component should be, so the filter did not see them.
    -- 123 survived on the reference database and are counted by every query
    -- that does not filter on type.
    DELETE FROM nodes WHERE type LIKE 'ComComponents%';

    -- 4. Edges whose endpoints no longer exist.
    --
    -- upgrade.sql deletes the nodes of every component that did not survive the
    -- port — todos being the large one — but never the edges pointing at them.
    -- 3,427 remained on the reference database, roughly a tenth of the table,
    -- each a relationship to a node that cannot be read.
    --
    -- Last, so that the repairs above have already salvaged everything
    -- salvageable and only genuine orphans are removed.
    DELETE e FROM edges e LEFT JOIN nodes n ON n.id = e.node_a_id WHERE n.id IS NULL;
    DELETE e FROM edges e LEFT JOIN nodes n ON n.id = e.node_b_id WHERE n.id IS NULL;
END;
-- +goose StatementEnd
CALL anahita_repair_legacy_edge_types();
DROP PROCEDURE anahita_repair_legacy_edge_types;

-- +goose Down
-- +goose StatementBegin
-- Not reversible, and deliberately so. Restoring a wrong type string onto an
-- edge, or resurrecting an edge to a node that does not exist, would put back
-- exactly the corruption this removes. The Down is a no-op rather than a lie.
SELECT 1;
-- +goose StatementEnd
