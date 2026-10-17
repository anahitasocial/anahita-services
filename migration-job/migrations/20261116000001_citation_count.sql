-- How many people cite a node, kept on it (the user, 2026-10-16: a post
-- cited more, and its author and owner, count for more in search and
-- recommendations). A citation is edge.medium.citation.v1, from an article
-- or a topic to a post or a profile it shows within itself.
--
--   nodes.citation_count  on a post: the people who cite it, each once,
--                         never its author. On a person, a group or an
--                         event: the people who cite it, its profile or
--                         any of its posts, never itself.
--
-- Only a citation in a published post that anybody may read counts. The
-- graph keeps it in step from here on (repositories/citations.go); this
-- counts what there is.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_citation_count()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'citation_count'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `citation_count` int(11) unsigned NOT NULL DEFAULT 0 AFTER `quote_count`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_citation_count();
DROP PROCEDURE anahita_add_citation_count;

-- +goose StatementBegin
UPDATE nodes cited
JOIN (
  SELECT cite.node_b_id AS id, COUNT(DISTINCT citing.created_by) AS total
  FROM edges cite
  JOIN nodes citing ON citing.id = cite.node_a_id
  JOIN nodes target ON target.id = cite.node_b_id
  WHERE cite.type = 'edge.medium.citation.v1'
    AND target.type LIKE 'node.medium.%'
    AND citing.created_by <> target.created_by
    AND citing.enabled = 1 AND citing.deleted_at IS NULL
    AND citing.draft = 0 AND citing.access = 'public'
  GROUP BY cite.node_b_id
) counted ON counted.id = cited.id
SET cited.citation_count = counted.total;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE nodes actor
JOIN (
  SELECT a.id AS id, COUNT(DISTINCT citing.created_by) AS total
  FROM nodes a
  JOIN nodes target ON target.id = a.id OR target.owner_id = a.id OR target.created_by = a.id
  JOIN edges cite ON cite.node_b_id = target.id AND cite.type = 'edge.medium.citation.v1'
  JOIN nodes citing ON citing.id = cite.node_a_id
  WHERE a.type LIKE 'node.actor.%'
    AND citing.created_by <> a.id
    AND citing.owner_id <> a.id
    AND citing.enabled = 1 AND citing.deleted_at IS NULL
    AND citing.draft = 0 AND citing.access = 'public'
  GROUP BY a.id
) counted ON counted.id = actor.id
SET actor.citation_count = counted.total;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_citation_count()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'citation_count'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `citation_count`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_citation_count();
DROP PROCEDURE anahita_drop_citation_count;
