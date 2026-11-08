# Muting profiles (roadmap row 133)

Planned on 2026-11-07 from FediDB's comparison with Mastodon ("Mute
accounts", answered "no" in [features.md](../features.md)); written on
2026-11-08, when the row was reached, with the user's choices of that day.
**Phase 1 (services) done 2026-11-08;** phase 2 (web) next.

## Contents

1. [Why](#why)
2. [What is settled](#what-is-settled)
3. [What a mute hides](#what-a-mute-hides)
4. [What a mute leaves alone](#what-a-mute-leaves-alone)
5. [Storage](#storage)
6. [Where it is checked](#where-it-is-checked)
7. [The API](#the-api)
8. [The web app](#the-web-app)
9. [Found on the way: a person's own Blocked list is empty](#found-on-the-way-a-persons-own-blocked-list-is-empty)
10. [Out of scope](#out-of-scope)
11. [Phases](#phases)
12. [Documentation](#documentation)

## Why

A block is for somebody one wants away: it ends the follows both ways, hides
each from the other, and is told to their server. Most of the time less is
wanted: a colleague who posts too much, a relative one follows out of
kindness, a thread one is tired of. Mastodon's mute is for that, and people
coming from Mastodon expect it, in its CSV too (row 124).

## What is settled

The user's choices, 2026-11-08:

1. **Their posts go out of sight everywhere but their own profile:** the
   Following feed, custom feeds, a hashtag's and a location's posts, search,
   what is trending, their replies under other posts, and their reposts.
   Visiting their profile shows it as it is.
2. **Their notifications too, by a tick,** "Hide their notifications",
   ticked to begin with, in the Mute dialog.
3. **Until unmuted, or for a time:** Until I unmute, 1 day, 1 week, 1 month.
4. **People, here and elsewhere,** as blocks are: on this site, on the
   fediverse and on Bluesky. A group or an event one would rather not see is
   unfollowed.

And, as Mastodon's:

1. **Nobody is told.** The muted person is not told and cannot find out:
   the list and the relationship are the muter's alone. Nothing is sent to
   their server, nothing is published to other services.
2. **Nothing ends.** Follows stay both ways: one can follow somebody and
   mute them, and they can go on following, liking and replying.

## What a mute hides

From the person who muted, while the mute lasts:

1. **Posts the muted person wrote,** wherever they are: on their profile, in
   a group or an event, on somebody else's profile.
2. **Posts on the muted person's profile,** written by others.
3. **Their reposts,** and others' reposts of their posts.
4. **Their replies** under anybody's post, and the replies under those,
   as a block hides them; the thread's reply count stays everybody's.
5. **Them in suggestions** (recommendation-service).
6. **Notifications they caused** (a like, a reply, a mention, a follow, a
   quote, a repost), when "Hide their notifications" was ticked. Those are
   not made while the mute lasts; what was told before stays.

## What a mute leaves alone

1. **Their profile,** visited: its feed, its tabs, its counts.
2. **A post opened by its address,** or from a notification, a bookmark or
   a quote: muting hides lists, not what was asked for by name.
3. **The people lists:** followers, following, search for people. A muted
   person is still found by name, to be unmuted.
4. **Bookmarks:** what one saved stays saved.
5. **Everybody else's view:** a mute is one person's.
6. **Counts:** likes, replies and followers count everybody.

## Storage

An edge, as a block is, with no new table and no migration:

| Column | Holds |
| --- | --- |
| `type` | `edge.actor.mute.v1` (`constants.EdgeMuteEntityType`) |
| `node_a_id` | Who muted |
| `node_b_id` | Who is muted |
| `meta` | `notifications` when their notifications are kept; empty when hidden |
| `ends_at` | When the mute ends; `NULL` until unmuted |

A mute whose `ends_at` has passed is over at once: every query asks for
`ends_at IS NULL OR ends_at > UTC_TIMESTAMP()`. graph-grpc-service's hourly
sweeper deletes those rows, so the list does not keep them. An account's
purge takes its mutes, both ways, with its other edges.

## Where it is checked

**Not in the access gate.** `relNotBlockedEitherWay` and the gates built on it
answer "may this person see this?", for single posts, notifications and
`FilterVisible` too. A mute is not about that, so it is a step of its own,
`SetMutedQuery` in `mutes.go`: one way only
(the viewer muted the author or the profile), and added to lists alone.

| List | Where |
| --- | --- |
| Following feed | `feed_leaders.go`, count and browse |
| A custom feed | `feed_custom.go` |
| A profile's feed | `feed_actor.go`, except the muted person's own: decided before the query is built |
| A hashtag's and a location's posts | `BrowseNeighbours` for medium edges, `BrowseOrderedByDegree`, `Trending` |
| Search, posts only | `search.go` |
| Replies | `replies.go`, `Thread` and the hidden replies (`selectHiddenReplies`) |
| Suggestions | recommendation-service, `signals.go`, `eligibleCandidate` |

For a repost the step takes the reposter and the reposted post's author and
profile; for a post, `node.created_by` and `node.owner_id`.

**Notifications** are decided when they are made. Every notification
notification-service makes goes through one check (`add`, in
`cmd/api/handlers/helpers.go`), which reads the relationship of its
recipient to whoever acted (`SocialgraphGRPC.Read`, which gains `IsMuting`
and `MuteHidesNotifications`); a recipient who muted them, with the tick, is
not told. When the graph cannot answer, the notification is made. Push (row
138) follows from that.

**The relationship** a viewer reads on a person (`actor-service`,
`person.go`, as `is_blocked_by_viewer` is) gains `is_muted_by_viewer` and
`mute_ends_at`.

## The API

socialgraph-service, beside the blocks:

| Request | Does | Allowed to |
| --- | --- | --- |
| `GET /socialgraph/:actor_id/mutes/?q=&limit=&start=` | The people this person muted, most lately first, with `mute_ends_at` and `mute_hides_notifications` | The person |
| `POST /socialgraph/:target_id/mutes/:source_id` `{ "hide_notifications": true, "for": "day" }` | Mutes, or changes a mute; `for` is `day`, `week`, `month` or empty for until unmuted | The person, of a person not themself and not blocked either way |
| `DELETE /socialgraph/:target_id/mutes/:source_id` | Unmutes | The person |

`SocialgraphMuteService` in graph-grpc-service, with `Add` and `Delete`, as
`SocialgraphBlockService`. No event is published: no service has anything
to do about a mute.

**The download** (export-service) gains `data/muted.json`, beside
`blocked.json`, and a Muted section on its page. Mastodon's CSV of mutes is
row 124's, with the others.

## The web app

1. **Mute,** in a profile's menu after Block, and in the menu of a post or
   reply by somebody else; Unmute there while muted, at once, with no
   dialog.
2. **The Mute dialog:** "Mute @alias?"; a line, "You won't see their posts
   or replies in your feeds. They won't be told, and can still follow you.";
   the tick, "Hide their notifications"; a select, "For": Until I unmute,
   1 day, 1 week, 1 month. Cancel and Mute, full width, side by side.
3. **Settings › Privacy › Muted,** a row under Blocked with how many, which
   opens a dialog as Blocked does: picture, name, until when ("Until
   15 Nov"), and Unmute on each; infinite scroll.
4. **After muting,** a list already loaded drops their cards; the profile
   itself is unchanged, its menu saying Unmute.
5. Every string in the five bases.

## Found on the way: a person's own Blocked list is empty

Tried on the dev stack on 2026-11-08: Worf blocked Dax, and
`GET /socialgraph/<worf>/blocks/` answered `total: 0`, `data: null`, with the
edge in MySQL. `BrowseNeighbours` gates every neighbour with
`SetActorAccessQuery`, whose `relNotBlockedEitherWay` hides from the
viewer everybody they blocked: the whole list. The list of mutes goes the
same way, so the fix comes first: a list of one's own blocks or mutes is
gated by the profile's access alone, not by the block, and `data` is `[]`
when it is empty. A test against the database keeps it. In the web app,
`permissions/actor.js` `canFollow` asks `actor.isBlocked`, which nothing
sends; it becomes `isBlockedByViewer`.

## Out of scope

1. Muting a thread (Mastodon's "Mute conversation"): Unfollow post, in a
   post's menu, does the notification half.
2. Muting a whole server for oneself, and word filters: rows of their own,
   if asked for ([features.md](../features.md)).
3. Muting groups and events.
4. Muting on Bluesky's side (`app.bsky.graph.muteActor`): Bluesky's app
   keeps its own.
5. Mastodon's CSV of mutes, in and out: row 124, through
   [imports.md](imports.md).

## Phases

1. **Services:** the Blocked list's fix; the edge, the gRPC service, the
   routes; `SetMutedQuery` in every list above; the relationship; notifications;
   the sweeper; the download; tests against a database for each list, and
   the visibility agreement test left as it is; tried on the dev stack:
   about a session. **Done 2026-11-08.**
2. **Web:** the menu items, the dialog, Settings › Privacy › Muted, the
   `canFollow` fix, strings in every base; tried at both sizes: half a
   session.

About a session and a half, services and web.

## Documentation

1. anahita-services: [features.md](../features.md) ("Mute accounts" to yes),
   [permissions.md](../permissions.md) (a section on mutes, after blocks),
   [privacy.md](../privacy.md), [feeds.md](../feeds.md),
   [notifications.md](../notifications.md), [the-graph.md](../the-graph.md)
   (the edge), [recommendations.md](../recommendations.md),
   [data-export.md](../data-export.md), the API reference, the plans table
   in [README.md](../README.md), `CHANGELOG.md`.
2. anahita-web: `docs/architecture.md` (the menu, the dialog, Settings ›
   Privacy › Muted), the roadmap, `CHANGELOG.md`.
