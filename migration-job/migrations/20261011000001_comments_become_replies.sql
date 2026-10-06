-- Comments become replies.
--
-- A comment was its own kind of node, under a post. A reply is a note that
-- answers a post: the same thing, and it can be answered in turn, liked,
-- tagged and searched like any note. From here there is one kind.
--
-- Each comment becomes a note that is a direct reply to the post it was
-- under:
--
--   type, service   a note, of text-service
--   owner_id        its author. A comment had no owner; a reply is owned by
--                   whoever wrote it
--   root_id         the post it was under. Comments were never nested, so
--                   the post it answers is also the top of its thread
--   parent_type     the post's type as it is now. Comments still carried the
--                   PHP application's name for it
--   access          empty: a reply has no audience of its own
--
-- Its id does not change, so a like, a hashtag or a mention that pointed at
-- the comment points at the reply. Only the type those edges record for it is
-- brought up to date.
--
-- TWO THINGS ARE CHANGED ON THE WAY, both decided on 2026-09-28:
--
--   - Comments from before 2016-01-30 were all switched off by the PHP
--     application's move to a newer version, about nine in ten of every
--     comment there is. Those under a post that is itself still published
--     are switched back on. Those under a post that is switched off stay as
--     they are.
--   - Comments whose post no longer exists are deleted, with the edges that
--     touch them. Nothing could show them: a reply is seen by whoever can
--     see its post.
--
-- Then every post's count is counted again from what is under it.
--
-- Running it again changes nothing: afterwards there are no comments left
-- for any statement here to find.
--
-- THERE IS NO WAY BACK FROM THIS BUT A BACKUP. A reply made after it cannot
-- be told from a comment turned into one. Down does nothing.

-- +goose Up

-- Comments under a post that is gone.
--
-- The edges at either end, as two statements. As one, joining on "this end
-- or that end", MySQL can use neither index and compares every edge with
-- every comment: five minutes on a database with 29,000 edges, against none
-- for the pair.
DELETE edge
FROM edges edge
JOIN nodes comment
  ON comment.id = edge.node_a_id
LEFT JOIN nodes post
  ON post.id = comment.parent_id
WHERE comment.type = 'node.base.comment-service.comment.v1'
  AND post.id IS NULL;

DELETE edge
FROM edges edge
JOIN nodes comment
  ON comment.id = edge.node_b_id
LEFT JOIN nodes post
  ON post.id = comment.parent_id
WHERE comment.type = 'node.base.comment-service.comment.v1'
  AND post.id IS NULL;

DELETE notification
FROM notifications notification
JOIN nodes comment
  ON comment.id = notification.comment_id
LEFT JOIN nodes post
  ON post.id = comment.parent_id
WHERE comment.type = 'node.base.comment-service.comment.v1'
  AND post.id IS NULL;

DELETE comment
FROM nodes comment
LEFT JOIN nodes post
  ON post.id = comment.parent_id
WHERE comment.type = 'node.base.comment-service.comment.v1'
  AND post.id IS NULL;

-- Switched-off comments under a post that is still published.
UPDATE nodes comment
JOIN nodes post
  ON post.id = comment.parent_id
SET comment.enabled = 1
WHERE comment.type = 'node.base.comment-service.comment.v1'
  AND comment.enabled = 0
  AND post.enabled = 1;

-- What points at a comment now points at a note.
UPDATE edges
SET node_a_type = 'node.medium.text-service.note.v1'
WHERE node_a_type = 'node.base.comment-service.comment.v1';

UPDATE edges
SET node_b_type = 'node.medium.text-service.note.v1'
WHERE node_b_type = 'node.base.comment-service.comment.v1';

-- The comments themselves.
UPDATE nodes comment
JOIN nodes post
  ON post.id = comment.parent_id
SET comment.type = 'node.medium.text-service.note.v1',
    comment.service = 'text-service',
    comment.owner_id = comment.created_by,
    comment.owner_type = IF(comment.created_by IS NULL, NULL, 'node.actor.person-service.person.v1'),
    comment.root_id = comment.parent_id,
    comment.parent_type = post.type,
    comment.access = '',
    comment.comment_status = 1,
    comment.comment_count = 0
WHERE comment.type = 'node.base.comment-service.comment.v1';

-- Every post's count, from the replies there now are under it. The same
-- count the graph keeps from here on: replies that are switched on, not
-- removed, and not hidden from the thread.
--
-- Every post that has any reply is counted, including one whose replies are
-- all switched off, which ends at zero. Counting only the posts with a reply
-- to count would leave those showing the number they had before.
UPDATE nodes post
JOIN (
  SELECT
    reply.root_id,
    SUM(
      reply.enabled = 1
      AND reply.deleted_at IS NULL
      AND NOT EXISTS (
        SELECT 1
        FROM edges hidden
        WHERE hidden.type = 'edge.medium.hidden-reply.v1'
          AND hidden.node_a_id = reply.root_id
          AND hidden.node_b_id = reply.id
      )
    ) AS replies
  FROM nodes reply
  WHERE reply.root_id IS NOT NULL
  GROUP BY reply.root_id
) said
  ON said.root_id = post.id
SET post.comment_count = said.replies;

-- And a post with nothing under it counts nothing. Some carried a number
-- from comments deleted long ago.
UPDATE nodes post
LEFT JOIN nodes reply
  ON reply.root_id = post.id
SET post.comment_count = 0
WHERE post.type LIKE 'node.medium.%'
  AND post.root_id IS NULL
  AND post.comment_count > 0
  AND reply.id IS NULL;

-- +goose Down
-- Deliberately nothing. See the note at the top.
SELECT 1;
