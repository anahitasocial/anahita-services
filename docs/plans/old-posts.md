# Deleting one's old posts (roadmap row 132)

Asked for by the user on 2026-11-07, from FediDB's list of features
("Automatically delete old posts", answered "no" in
[features.md](../features.md)). Planned the same day. Not built.

## Contents

1. [Why](#why)
2. [What is settled](#what-is-settled)
3. [What a person chooses](#what-a-person-chooses)
4. [What is deleted, and what is not](#what-is-deleted-and-what-is-not)
5. [Where it runs](#where-it-runs)
6. [Storage and the API](#storage-and-the-api)
7. [Out of scope](#out-of-scope)
8. [Phases](#phases)
9. [Documentation](#documentation)

## Why

Some people want what they say to last a while, not for good: a journalist
whose old posts are quoted against them out of their time, somebody who
posts about their day and not for an archive. Mastodon gives each person
that choice, and FediDB asks for it.

## What is settled

1. **Each person's own choice, off for everybody.** Nothing is ever deleted
   for being old unless its author asked: old and useful counts as much as
   new, and an operator never deletes people's posts by a setting
   ([remote-content-limits.md](remote-content-limits.md), "Asked by the
   user").
2. **A delete like any other.** Each post goes the way a person's own
   Delete takes it, so that the fediverse is sent its `Delete`, Bluesky's
   record is removed, and its files go with it.
3. **Nothing goes by surprise.** Turning it on says how many posts would go
   first, and nothing is deleted until a day after it is turned on or
   changed.

## What a person chooses

Under Settings › Privacy, one card, **Delete old posts**:

1. **Off**, or older than a month, three months, six months, a year or two
   years.
2. **Keep** (all ticked at first):
   1. pinned posts;
   2. posts they saved themselves;
   3. articles;
   4. posts with at least a number of likes and reposts together, 10 at
      first.
3. A line under it: "143 posts would be deleted tomorrow. Posts are deleted
   on the fediverse and Bluesky too, and cannot be brought back."

## What is deleted, and what is not

1. **Deleted:** notes, photos, videos and topics on their own profile, and
   their replies anywhere, older than the age chosen, but what is kept.
2. **Not deleted:** what they posted in a group or an event, which is part
   of what that group or event keeps; they can delete those by hand, as
   now.
3. **Their age** is when the post was made, not when it was last edited.
4. **A thread** loses its notes one by one as each passes the age, the first
   one first.

## Where it runs

In graph-grpc-service's hourly sweeper (`sweeper/`), under its lock: for each
person who has it on, a few hundred posts at a time, oldest first, so that a
long backlog is worked off over several runs and never holds up the site.
Each delete publishes the event a person's own does, which the activitypub
and atproto services already follow.

## Storage and the API

1. Two columns on a person's node: `delete_posts_after` (days, 0 for off)
   and `delete_posts_keep` (the exceptions, as a set), with
   `delete_posts_from`, the day it may start. One migration, in `init.sql`
   and `upgrade.sql` too.
2. `GET` and `PATCH /people/:id/old-posts` `{ after_days, keep: [...],
   min_interactions }`; the `GET` answers with `would_delete`, how many
   posts the next run would take.

## Out of scope

1. An operator's setting deleting people's own posts.
2. Deleting old likes, reposts or saves.
3. Keeping a copy of what is deleted: the download of one's data
   (export-service) is there to make one first, and the card says so.

## Phases

1. **Services:** the columns, the API with its count, the sweeper's part,
   tested against a database, and tried on the dev stack with the
   fediverse lab: about a session.
2. **Web:** the card, its line, in every base language; tried at both
   sizes: half a session to a session.

About two sessions in all, services and web.

## Documentation

1. anahita-services: [features.md](../features.md) (FediDB's answer to
   "yes"), `docs/account-lifecycle.md` (what the sweeper does), the API
   reference, `CHANGELOG.md`.
2. anahita-web: `docs/architecture.md` (Settings › Privacy), `CHANGELOG.md`.
