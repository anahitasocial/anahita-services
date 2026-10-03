# Profile tabs for replies and reposts

*Roadmap item 24, planned 2026-10-02. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
A person's profile should separate what they wrote from what they replied to and what they reposted: a **Posts** tab, a **Replies** tab and a **Reposts** tab. The person chooses whether their reposts also appear in their main (Posts) feed.

## What exists
- The profile feed is `GET /feeds/actor/:actor_id/` (`feed-service/cmd/api/handlers/feed_actor.go`, graph-grpc `feed_actor.go`). It already takes `IncludeReposts` (`feed_actor.go:46,81`), which adds `node.base.repost-service.repost.v1` nodes to the query.
- A repost is a node owned by the reposter (`graph-grpc-service/cmd/server/handlers/repost.go:111`).
- Replies don't exist yet as notes. They arrive with item 7 (notes as replies), whose design already says profile lists show top-level notes by default and that a profile can offer a "Posts & replies" tab.
- The web profile's tabs come from `TAB_COMPONENTS` (`anahita-web/src/constants/actor.js`), one per media type.

## Decisions
- **Three tabs: Posts, Replies, Reposts.** The per-media-type tabs (Photos, Articles, …) stay as they are.
- **A person setting, "Show my reposts in my Posts tab"**, on by default (the Mastodon and Bluesky behaviour). Turning it off moves reposts to the Reposts tab only.
- **The setting only affects the profile.** Followers still receive reposts in their home feed; delivering a post to followers is what a repost is for. Followers who don't want them can already exclude reposts in a custom feed (item 14).
- Groups get the same tabs. A group's setting is changed by its admins.

## Design
**API**
- `GET /feeds/actor/:actor_id/?filter=posts|replies|reposts` (default `posts`):
  - `posts`: top-level posts (`parent_id IS NULL`), plus reposts when the actor's setting is on. Pinned posts first (item 15).
  - `replies`: the actor's replies (`parent_id IS NOT NULL`, item 7), each returned with a short excerpt of the post it answers so it reads in context. The reply is hidden when the viewer can't see its thread's root.
  - `reposts`: the actor's reposts, newest first, each with the original post. A repost is hidden when the viewer can't see the original.
- The existing `IncludeReposts` request flag stays for the leaders feed. For the actor feed, the server decides from `filter` and the actor's setting, so a client can't override the person's choice.
- **Setting:** `show_reposts_on_profile` on the actor (a boolean in the actor's settings `meta`, default true), edited through the existing profile edit endpoint and returned on the actor.
- The same access gate as every feed applies (item 0).

**Federation (item 8)**
- ActivityPub: the outbox keeps listing `Announce` activities whatever the setting says; it is a display preference on this site.
- Remote actors' profiles get the same three tabs from their stored posts.

## Web (anahita-web)
- Profile tabs: **Posts**, **Replies**, **Reposts**, then the media-type tabs. Each is a URL (`/people/:alias/replies`, `/people/:alias/reposts`) so it can be linked.
- Replies tab: each reply shows "Replying to @name" with the excerpt, linking to the thread.
- Reposts tab: the original post with "Reposted by …" above it, as in the home feed.
- Settings → Account (and group Settings): a switch, "Show my reposts in my Posts tab".
- i18n in en-GB and fr-FR.

## Estimate

| Part | Time |
| --- | --- |
| Feed filter (posts, replies, reposts), reply context excerpt, setting on the actor, tests | 3–4 h |
| Web: three tabs with routes, reply and repost rows, the settings switch | 3–4 h |
| **Total** | **about 6–8 h, 2 sessions** |

## Verification
- With the setting on, a repost appears in both Posts and Reposts. With it off, only in Reposts, and followers still see it in their home feed.
- Sending `IncludeReposts=true` on the actor feed while the setting is off does not bring reposts back into Posts.
- A reply to a followers-only post doesn't show in the Replies tab for a stranger.
- A repost of a post the viewer can't see is omitted.
- The tab URLs load directly and survive a refresh.

## Dependencies
- **Hard:** item 7 (replies as notes) for the Replies tab. The Reposts tab and the setting can ship before it.
- **Soft:** item 0 (feed access gate), item 15 (pinned posts lead the Posts tab), item 14 (followers filtering reposts in their own feeds).

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `architecture.md`**: the actor feed's `filter` values and the reposts setting.
- **services `permissions.md`**, *Reposts*: what the setting does and doesn't affect.
- **web `architecture.md`**, *Routes*: the Replies and Reposts profile routes.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-02 |
| Implementation | not started |
