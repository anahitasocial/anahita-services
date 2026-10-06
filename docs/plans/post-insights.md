# Post insights: views, replies, reposts, quotes and likes

> **Set aside on 2026-10-05.** The user decided against insights: no view
> counts, daily or otherwise. What was built instead is a **post's activity**,
> the counts of likes, reposts, quotes and replies with who did each, from
> lists that already existed plus `GET /notes/:id/quotes`. See
> `docs/architecture.md`, A post's activity. This plan is kept for the
> reasoning about views, should they ever be wanted.

*Roadmap item 22, planned 2026-10-02. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
Every medium node (note, article, topic, photo, video) should have an insight view showing how many times it was viewed, replied to, reposted, quoted and liked, with the lists of people who liked and reposted it.

Profile-level analytics (how many visited the profile, how many went on to follow) are **not** part of this item. See *Decisions*.

## What exists
- **Counts** are already stored on each node: `vote_up_count`, `comment_count`, `repost_count`, and `quote_count` (in `migration-job/schema/init.sql`; item 25 starts writing `quote_count`).
- **Lists** exist, for signed-in viewers only:
  - likers: `GET /likes/:node_id/` (`like-service/cmd/api/routes.go:28`);
  - reposters: `GET /reposts/:parent_id/` (`feed-service/cmd/api/routes.go:37`).
- **Views are not counted.** `nodes.view_count` exists and is returned as `view_count` on media, but no service ever increments it. It was the legacy `hits` column, renamed on 2026-10-02 (`done/remove-unused-node-columns.md`).
- **Web:** `src/containers/likes/` has a likes list. There is no insights view and no reposters list.

## Decisions
- **View counts are a plain number, never a list.** Nobody, including the author, can see *who* viewed a post. This keeps the rule set in the pinned-posts plan (item 15): no view tracking of people.
- **The view count is shown to the author only** (and to the admins of the profile or group the post is on, and site admins). Likes, reposts, quotes and replies stay visible to everyone who can see the post, as today and as on Mastodon and Bluesky.
- **No profile analytics.** Profile visits and "viewed, then followed" funnels need per-person tracking over time, which is the surveillance this project avoids, and no fediverse server offers it. A profile already shows its follower count.

## Design
**Counting views**
- A view is counted when a signed-in person opens the post's own page (`GET /{ns}/:id`), not when it scrolls past in a feed. Feed impressions would need the client to report what was on screen, which is more tracking for a less meaningful number.
- **One view per person per post per day.** A Redis key `view:<node_id>:<hash(viewer_id + daily salt)>` with a 24 h TTL decides whether to count. The salt rotates daily and the key expires, so no lasting record of who viewed what exists anywhere.
- The author's own views are not counted. Logged-out views are counted once per IP hash per day, with the same expiring key.
- Increments go to a Redis counter and are flushed to `nodes.view_count` in batches by the sweeper (every few minutes), so a popular post doesn't cause a database write per view.
- Without Redis (cookie sessions), views are not counted and the insight view says so. No silent wrong numbers.
- **Federation (item 8):** views on other servers can't be counted. The insight view labels the number "Views on this site".

**Insights endpoint**
- `GET /{ns}/:id/insights`, in each media service through a shared handler in `anahita-libs`:
  ```json
  { "views": 412, "likes": 37, "replies": 9, "reposts": 5, "quotes": 2,
    "first_viewed_at": "…", "views_by_day": [ { "day": "2026-10-01", "views": 120 }, … ] }
  ```
- `views` and `views_by_day` are returned only to the author, the profile's admins and site admins. Everyone else who can see the post gets the other counts.
- `views_by_day` covers the last 30 days, from a small table `node_views_daily(node_id, day, views)` that the same flush writes. It holds totals only.
- **Lists:** the existing likes and reposts endpoints, plus `GET /{ns}/:id/quotes` from item 25. Each list applies blocks and actor access for the viewer, so a liker whose profile the viewer can't see is omitted, and the count next to the list says "37 likes" while the list may show fewer.

**Remote likes and reposts (item 8, phase 2)** appear in the same counts and lists once federation delivers them.

## Web (anahita-web)
- **"Insights"** in the post menu (`MediaMenu`) and on the read page, opening a dialog or side panel:
  - a row of counts: Views (author only), Likes, Replies, Reposts, Quotes;
  - for the author, a small 30-day views chart;
  - tabs with the lists: **Likes**, **Reposts**, **Quotes**. Each row is an actor with a Follow button; Quotes rows show the quoting post.
- Tapping the like or repost count on any post opens the same panel on that tab.
- The existing `containers/likes/Browse.jsx` becomes the Likes tab; add a matching reposters list.
- i18n in en-GB and fr-FR.

## Estimate

| Part | Time |
| --- | --- |
| View counting: Redis dedupe and counter, sweeper flush, `node_views_daily`, tests | 3–4 h |
| Insights endpoint and visibility rules, list filtering | 1–2 h |
| Web: insights panel, counts, chart, list tabs | 3–4 h |
| **Total** | **about 7–10 h, 2–3 sessions** |

## Verification
- Opening a post twice in a day as the same person counts one view; the author's own visits count none.
- No table or key maps a viewer to a post after 24 h; the Redis keys expire and `node_views_daily` holds only totals.
- A non-author gets no `views` field. The author, a group admin (for a group post) and a site admin do.
- The Likes tab hides a liker the viewer has blocked; the count is unchanged.
- With Redis off, the panel shows "Views aren't counted on this site" rather than 0.

## Dependencies
- **Soft:** item 7 (replies replace comments in the count), item 25 (quotes count and list), item 8 phase 2 (remote likes and reposts), item 15 (the no-tracking rule this follows).

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `architecture.md`**: how views are counted (daily dedupe, no viewer records) and the insights endpoint.
- **services `permissions.md`**: who sees view counts, and who sees the likes, reposts and quotes lists.
- **services `configuration.md`**: the flush interval, and that view counting needs Redis.
- **web `architecture.md`**: the insights panel.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-02 |
| Implementation | not started |
