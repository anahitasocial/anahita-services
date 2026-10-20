-- Every location and hashtag is for everybody (the user, 2026-10-19). Some
-- were made with no audience: three locations and most hashtags from the
-- PHP site, and the locations location-service made until now. A location's
-- list on a profile asks for an audience, so those showed to administrators
-- only.
--
-- Numbered after 20261122000001, the last folded migration, as every
-- migration is from here (migrations/README.md).

-- +goose Up
UPDATE nodes
   SET access = 'public'
 WHERE type IN ('node.tag.location-service.location.v1', 'node.tag.hashtag-service.hashtag.v1')
   AND (access IS NULL OR access = '');

-- +goose Down
-- Nothing: which tags had no audience is not kept, and none should.
SELECT 1;
