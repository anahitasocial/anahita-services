# Sweeping away unused hashtags and locations

anahita-web roadmap row 105. Planned 2026-10-27, for banu.io's launch (its
row 21), and for any site after a purge.

## Why

A hashtag or a location is a node of its own, made the first time a post
carries it. Nothing removes it when the last post that carried it is
deleted or purged, so it stays: in `GET /hashtags/_suggest` as somebody
types, in search, and in counts, with no post behind it.

On anahita.io's archive, upgraded (2026-10-27), before any purge:

| Leftover | Count |
| --- | --- |
| Hashtags no post is tagged with | 554 of 1,773 |
| Locations no post is tagged with | 5 of 75 |
| Nodes whose owner, parent or author is missing | 0 |
| Edges with a missing end | 0 |

The purge removes each account's edges and children with it, so the graph
itself stays whole; what it leaves are tags. banu.io's purge of 307 people
and 11 groups will leave many more.

## What counts as unused

| Kind | Unused when | Kept anyway when |
| --- | --- | --- |
| Hashtag (`node.tag.hashtag-service.hashtag.v1`) | No `edge.tag.hashtag.v1` leaves it | Made in the last day, so that a post being written is never raced |
| Location (`node.tag.location-service.location.v1`) | No `edge.tag.location.v1` leaves it | Made in the last day; or named by a custom feed's `locations_any`, which names locations by id |

A custom feed names hashtags by their name, not their id, so removing an
unused hashtag changes no feed: a post tagged with it later makes it again,
and the feed finds it. Following a hashtag is having it in a custom feed.

## How

- **In the sweeper job,** `graph-grpc-service/cmd/sweeper`, which already
  runs every ten minutes as a CronJob (`k8s/base/jobs/sweeper-cronjob.yml`)
  and then exits: a second step after the accounts whose grace period has
  ended. One process at a time, so two sweeps never overlap.
- **In batches** of 500, oldest first, each in its own transaction, the
  "still unused" test repeated in the `DELETE` itself, so that a post
  tagged between the read and the delete keeps its tag.
- **Through graph-grpc-service's repository,** as the purge removes nodes,
  so that what else names a node goes with it; no SQL outside it.
- **Logged** as a count of each kind removed, not their names.
- **A flag for a run by hand,** `sweeper -tags-only`, to see a sweep's
  counts at once, as banu.io's row 21 will after its purges.

## Tests

- A hashtag with a post keeps; one without, older than a day, goes; one
  made an hour ago keeps.
- A location in a custom feed keeps although no post carries it.
- A post tagged while the sweep runs keeps its tag.
- After a purge in the integration test, the sweep leaves no tag without a
  post, and the four "missing" counts above stay 0.

## Documentation

- `docs/hashtags.md`, which covers both: when an unused one goes.
- `docs/account-lifecycle.md`, "Purging now": what is left after a purge,
  and the sweep that follows.
- `docs/operating.md`: the sweeper job's two steps, and `-tags-only`.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-27 |
| Implementation | not begun |
