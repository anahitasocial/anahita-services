-- Somebody elsewhere who approves their followers is asked, not followed
-- (anahita-web roadmap row 130). Data only; the schema is as it was.
--
-- 1. A profile kept from the fediverse says whether it approves its
--    followers in `allow_follow_request`, from what its server said when it
--    was last fetched (`activitypub_actors.locked`). Until now it was kept
--    only there, where the rules could not see it.
-- 2. A follow sent to such a profile and still waiting for its answer
--    becomes the request it always was: not counted among their followers,
--    and "Requested" on the button. Their Accept, when it comes, makes it a
--    follow, as for a request made from now on.
-- 3. The counts of those it touched are made again.

-- +goose Up
UPDATE nodes n
JOIN activitypub_actors a
  ON a.node_id = n.id
SET n.allow_follow_request = a.locked
WHERE n.uri IS NOT NULL
  AND n.type = 'node.actor.activitypub-service.person.v1';

-- IGNORE: a request already there for the same two stays, and the follow
-- beside it is removed below.
UPDATE IGNORE edges e
JOIN activitypub_follows f
  ON f.local_id = e.node_a_id
 AND f.remote_id = e.node_b_id
 AND f.direction = 'out'
 AND f.state = 'pending'
JOIN activitypub_actors a
  ON a.node_id = f.remote_id
 AND a.locked = 1
SET e.type = 'edge.actor.follow-request.v1'
WHERE e.type = 'edge.actor.follow.v1';

DELETE e
FROM edges e
JOIN activitypub_follows f
  ON f.local_id = e.node_a_id
 AND f.remote_id = e.node_b_id
 AND f.direction = 'out'
 AND f.state = 'pending'
JOIN activitypub_actors a
  ON a.node_id = f.remote_id
 AND a.locked = 1
WHERE e.type = 'edge.actor.follow.v1';

UPDATE nodes n
JOIN activitypub_actors a
  ON a.node_id = n.id
 AND a.locked = 1
SET n.follower_count = (
  SELECT COUNT(*)
  FROM edges e
  WHERE e.type = 'edge.actor.follow.v1'
    AND e.node_b_id = n.id
);

UPDATE nodes n
JOIN (
  SELECT DISTINCT local_id
  FROM activitypub_follows
  WHERE direction = 'out'
) f
  ON f.local_id = n.id
SET n.leader_count = (
  SELECT COUNT(*)
  FROM edges e
  WHERE e.type = 'edge.actor.follow.v1'
    AND e.node_a_id = n.id
);

-- +goose Down
-- Nothing is put back: a request waiting is what a follow to somebody who
-- approves their followers was, and turning it into a follow again would
-- count somebody among their followers before they said yes.
SELECT 1;
