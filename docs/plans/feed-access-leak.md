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

## What was built (2026-10-02)

The fix went further than the plan, because the feed gate had a second fault and closing the first exposed a third.

- **The feed gate is no longer a separate copy.** `SetFeedAccessQuery` (`access_feed.go`) now dispatches by row type to the gates the rest of the package uses: posts to `mediumAccessCondition`, comments to `commentAccessCondition`, reposts to a new `repostAccessCondition`. The `object` join is gone from both feed queries.
- **Reposts leaked too.** A repost was judged by the original's access, but "is the viewer a follower" was asked of the reposter. Following somebody who reposted a followers-only post was enough to see it. A repost is now shown only when the original passes the medium gate for the viewer, and the reposter's profile admits them.
- **Owner and author rules in the list gate.** `permissions.MediumPermissions.CanRead` lets the profile's owner read everything on it, and the author read their own post while they can see the profile. `mediumAccessCondition` had neither rule, so nobody could list their own followers-only or mutuals-only posts. The leak hid this in feeds; with the leak closed it would have shown. The list gate now has both rules.
- **One gate function for both columns:** `accessGate(column, viewer)` in `access_medium.go` replaces the two written-out copies (owner and node) in each of the medium and feed gates.
- **Tests** (`access_integration_test.go`, real MySQL): `TestActorFeedHonoursAPostsOwnAccess`, `TestLeadersFeedJudgesRepostsAndCommentsByTheirPost`, `TestAuthorSeesTheirOwnPostInAGroup`. Against the old gate, 14 of their checks fail.

### Follow-ups, not done here
- `access_search.go`, `access.go` (`SetAccessQuery`) and `access_polymorphic.go` are further hand-written copies of the gate. They check a post's own access, so they don't leak, but they lack the owner and author rules. Moving them onto `accessGate` would remove the remaining copies.
- `nodes.status` is nullable in the schema but scanned as a plain string in `helpers.go`. Rows the app creates always set it; a legacy row with a NULL status would fail to load.
- The manual check on a running stack (logged out, a profile with a followers-only post) is still to do after the next deploy of graph-grpc-service.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | done 2026-10-02, on branch `feed-access-leak` |
| Documentation | `docs/permissions.md`, *Seeing posts* |
| Manual check on a running stack | to do |
