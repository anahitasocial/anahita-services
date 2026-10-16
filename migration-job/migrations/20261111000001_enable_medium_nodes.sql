-- Switches on every post, reply, photo and video that is switched off.
--
-- Nothing in the services switches a post off: a post is shown, hidden from a
-- thread, deleted or purged, and `enabled` is no longer one of those. What is
-- switched off came from the PHP site, where 12,460 of the 18,821 posts on the
-- development database were, all written from 2010 to 2015, by people whose
-- accounts are on. They read as real conversations, and the user decided on
-- 2026-10-16 that there should be no switched-off posts. Left as they were,
-- they were in no list and on no page, and a reply counted under nothing.
--
-- Each post's count of replies is worked out again, as legacy/upgrade.sql
-- counts it: the replies that are switched on, not removed, and not hidden
-- from the thread.
--
-- A removed post (deleted_at) stays as it is. Idempotent: a second run finds
-- nothing switched off. legacy/upgrade.sql switches them on too, for a site
-- upgraded from the PHP one, which never runs this.
--
-- Down does nothing: which posts were switched off is not kept.

-- +goose Up
-- +goose StatementBegin
UPDATE nodes
SET enabled = 1
WHERE type LIKE 'node.medium.%'
  AND enabled = 0
  AND deleted_at IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
