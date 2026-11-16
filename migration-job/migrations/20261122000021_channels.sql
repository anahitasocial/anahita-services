-- +goose Up
-- Websites become channels (roadmap row 147, phase 1): the rename alone.
-- Every column that holds a profile's type, the two tables of a channel's
-- feed, the choice of being suggested channels, and the custom feeds whose
-- definitions name the kind.

UPDATE nodes SET type = 'node.actor.actor-service.channel.v1'
WHERE type = 'node.actor.actor-service.website.v1';
UPDATE nodes SET owner_type = 'node.actor.actor-service.channel.v1'
WHERE owner_type = 'node.actor.actor-service.website.v1';
UPDATE nodes SET parent_type = 'node.actor.actor-service.channel.v1'
WHERE parent_type = 'node.actor.actor-service.website.v1';

UPDATE edges SET node_a_type = 'node.actor.actor-service.channel.v1'
WHERE node_a_type = 'node.actor.actor-service.website.v1';
UPDATE edges SET node_b_type = 'node.actor.actor-service.channel.v1'
WHERE node_b_type = 'node.actor.actor-service.website.v1';

UPDATE notifications SET subject_type = 'node.actor.actor-service.channel.v1'
WHERE subject_type = 'node.actor.actor-service.website.v1';
UPDATE notifications SET object_type = 'node.actor.actor-service.channel.v1'
WHERE object_type = 'node.actor.actor-service.website.v1';
UPDATE notifications SET target_type = 'node.actor.actor-service.channel.v1'
WHERE target_type = 'node.actor.actor-service.website.v1';

UPDATE node_deletion_logs SET node_type = 'node.actor.actor-service.channel.v1'
WHERE node_type = 'node.actor.actor-service.website.v1';
UPDATE purge_requests SET actor_type = 'node.actor.actor-service.channel.v1'
WHERE actor_type = 'node.actor.actor-service.website.v1';
UPDATE abuse_cases SET target_type = 'node.actor.actor-service.channel.v1'
WHERE target_type = 'node.actor.actor-service.website.v1';

-- A custom feed of channels said "website" among its kinds of profile.
UPDATE nodes SET meta = REPLACE(meta, '"website"', '"channel"')
WHERE type = 'node.feed.feed-service.generator.v1'
  AND meta LIKE '%"website"%';

RENAME TABLE websites TO channel_feeds, website_items TO channel_feed_items;
ALTER TABLE channel_feed_items
    RENAME COLUMN website_id TO channel_id,
    RENAME INDEX website_state TO channel_state;

ALTER TABLE recommendation_preferences
    RENAME COLUMN suggest_websites TO suggest_channels;

-- +goose Down
ALTER TABLE recommendation_preferences
    RENAME COLUMN suggest_channels TO suggest_websites;

ALTER TABLE channel_feed_items
    RENAME COLUMN channel_id TO website_id,
    RENAME INDEX channel_state TO website_state;
RENAME TABLE channel_feeds TO websites, channel_feed_items TO website_items;

UPDATE nodes SET meta = REPLACE(meta, '"channel"', '"website"')
WHERE type = 'node.feed.feed-service.generator.v1'
  AND meta LIKE '%"channel"%';

UPDATE abuse_cases SET target_type = 'node.actor.actor-service.website.v1'
WHERE target_type = 'node.actor.actor-service.channel.v1';
UPDATE purge_requests SET actor_type = 'node.actor.actor-service.website.v1'
WHERE actor_type = 'node.actor.actor-service.channel.v1';
UPDATE node_deletion_logs SET node_type = 'node.actor.actor-service.website.v1'
WHERE node_type = 'node.actor.actor-service.channel.v1';

UPDATE notifications SET target_type = 'node.actor.actor-service.website.v1'
WHERE target_type = 'node.actor.actor-service.channel.v1';
UPDATE notifications SET object_type = 'node.actor.actor-service.website.v1'
WHERE object_type = 'node.actor.actor-service.channel.v1';
UPDATE notifications SET subject_type = 'node.actor.actor-service.website.v1'
WHERE subject_type = 'node.actor.actor-service.channel.v1';

UPDATE edges SET node_b_type = 'node.actor.actor-service.website.v1'
WHERE node_b_type = 'node.actor.actor-service.channel.v1';
UPDATE edges SET node_a_type = 'node.actor.actor-service.website.v1'
WHERE node_a_type = 'node.actor.actor-service.channel.v1';

UPDATE nodes SET parent_type = 'node.actor.actor-service.website.v1'
WHERE parent_type = 'node.actor.actor-service.channel.v1';
UPDATE nodes SET owner_type = 'node.actor.actor-service.website.v1'
WHERE owner_type = 'node.actor.actor-service.channel.v1';
UPDATE nodes SET type = 'node.actor.actor-service.website.v1'
WHERE type = 'node.actor.actor-service.channel.v1';
