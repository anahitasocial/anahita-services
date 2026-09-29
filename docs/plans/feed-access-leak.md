# Fix the feed access leak

*Roadmap item 0, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
`graph-grpc-service/data/repositories/access_feed.go:70-72` checks only `object.access`. `object` is the row joined through `node.parent_id` (`feed_actor.go:45,84`, `feed_leaders.go:52,132`). A top-level post has no parent, so `object.access IS NULL` passes and the post's own `access` is never checked. As a result, every restricted top-level post (followers-only, leaders, mutuals, myself, and the legacy private notes) shows up in profile and home feeds, even to logged-out visitors. `feed-service` doesn't filter the results again.

## Implementation
- Gate on the effective access: `COALESCE(object.access, node.access)`. That is the parent's access for comments, and the node's own access for top-level posts.
- Run it through the same keyword logic `mediumAccessCondition` uses (`access_medium.go:62-157`: public, registered, followers, leaders, mutuals, admins, myself, plus the owner and admin bypass). Share that code rather than copying it.
- Check both feed queries (actor feed and leaders feed) and any other caller of `access_feed.go`.
- Tests: add to `access_integration_test.go` (next to `TestCommentsFollowTheirPost`, :209). A top-level `myself` post and a `followers` post must be hidden from a stranger and from a logged-out viewer in both feeds. They stay visible to the owner and, for `followers`, to a follower.

## Estimate
2–3 h.

## Verification
- The integration tests pass.
- Manual check: logged out, open a profile whose feed contains a followers-only post; the post is gone. Logged in as a follower, the post is visible.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `permissions.md`**, *Access levels*: state that feeds judge a post by its own access, and a reply by its parent's.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | not started |
