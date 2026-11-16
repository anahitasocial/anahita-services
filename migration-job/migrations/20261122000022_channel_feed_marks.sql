-- +goose Up
-- A channel's admins post as it (roadmap row 147, phase 3): a channel's own
-- post is no longer always made of its feed. The notes its feed made are
-- marked so in their meta (`from_feed`), as the worker marks each new one,
-- and stay as their site has them; what its admins write is theirs to
-- change. Data only: the schema is unchanged.

UPDATE nodes AS n
JOIN channel_feed_items AS i ON i.node_id = n.id
SET n.meta = IF(
    JSON_VALID(n.meta),
    JSON_SET(n.meta, '$.from_feed', TRUE),
    JSON_OBJECT('from_feed', TRUE)
)
WHERE n.owner_type = 'node.actor.actor-service.channel.v1';

-- +goose Down
UPDATE nodes AS n
JOIN channel_feed_items AS i ON i.node_id = n.id
SET n.meta = JSON_REMOVE(n.meta, '$.from_feed')
WHERE n.owner_type = 'node.actor.actor-service.channel.v1'
  AND JSON_VALID(n.meta);
