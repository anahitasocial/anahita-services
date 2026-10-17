-- A post naming a profile is a mention, not a citation (the user,
-- 2026-10-17: "only medium to medium is a citation; medium to actor is a
-- mention"). A profile embedded in an article, its address on a line of its
-- own, was kept as edge.medium.citation.v1 from the post to the profile;
-- from here on actor-service keeps it as edge.tag.mention.v1, from the
-- profile to the post, as an @handle is.
--
-- Each such citation becomes that mention, and goes. Nobody is told: these
-- are not new. Then the citations of every profile are counted again, now
-- only those of the posts it owns or wrote (repositories/citations.go).
--
-- Data only: no table changes, so nothing in schema/init.sql or
-- legacy/upgrade.sql.

-- +goose Up
-- +goose StatementBegin
INSERT IGNORE INTO edges (type, node_a_id, node_a_type, node_b_id, node_b_type, created_at, created_by, updated_at, modified_by)
SELECT 'edge.tag.mention.v1',
       cite.node_b_id,
       cite.node_b_type,
       cite.node_a_id,
       cite.node_a_type,
       COALESCE(cite.created_at, NOW()),
       cite.created_by,
       NOW(),
       cite.created_by
FROM edges cite
JOIN nodes profile ON profile.id = cite.node_b_id
WHERE cite.type = 'edge.medium.citation.v1'
  AND profile.type LIKE 'node.actor.%';
-- +goose StatementEnd

-- +goose StatementBegin
DELETE cite
FROM edges cite
JOIN nodes profile ON profile.id = cite.node_b_id
WHERE cite.type = 'edge.medium.citation.v1'
  AND profile.type LIKE 'node.actor.%';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE nodes
SET citation_count = 0
WHERE type LIKE 'node.actor.%'
  AND citation_count <> 0;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE nodes actor
JOIN (
  SELECT a.id AS id, COUNT(DISTINCT citing.created_by) AS total
  FROM nodes a
  JOIN nodes target ON target.owner_id = a.id OR target.created_by = a.id
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
-- Nothing to undo: which mentions were citations is not kept.
