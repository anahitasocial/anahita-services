# Writing a thread in one go

*Roadmap row 101, planned 2026-10-19. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

**Status:** planned, not started. After row 100.

## Context

The user, 2026-10-19: "There is nothing worse than a disorganized and broken
thread in the middle of a heated discussion or conversation, particularly
political discourse during election times." Threads has an expanded
composer for replies in which a person writes a thread of replies and posts
them in one go.

A long reply going to Bluesky is cut at 300 letters (row 100 links to the
rest). A thread written as a thread reads whole everywhere.

## Decisions

Settled with the user on 2026-10-19:

1. **One go, in order, all or nothing.** The parts are posted together: if
   one fails, none is posted. They go out to other networks in order, each
   a reply to the one before.
2. **Written here as a thread,** not split by the server: the writer decides
   where each part ends.

**Open, to settle before building:**

1. Numbered "1/3" only on other networks, or here too? Proposed: neither;
   the thread shows itself.
2. The most parts: proposed 10.
3. Top-level posts too, or only replies? Proposed: both, from the composer
   and from Reply.

## Design

1. **One request** (text-service): `POST` a list of parts, each a note with
   its own pictures and descriptions, made in one transaction; each part a
   reply to the one before, the first a reply to the post answered (or a
   post). One event for the thread, so federation sends them in order.
2. **Going out** (activitypub-service, atproto-service): the parts in order,
   each `inReplyTo` / `reply.parent` the one before, waiting for each to be
   accepted before the next. A part that fails to go out is retried in its
   place, never skipped.
3. **The composer** (web): "Add to thread" under a part makes the next; each
   part counts its letters against the networks the profile goes to (row
   100's line); the whole posts with one button. On a phone, the whole
   screen.
4. **Notifications:** one for the thread, not one per part.

## Checking

A thread of three with pictures, as a reply and as a post, from a federating
test profile; a failure in the second part leaves nothing posted. What went
out read on the PDS and the outbox, in order.

## Docs to change

`docs/api.md` and `docs/api/text-service.json`, `docs/federation.md`,
anahita-web `docs/` for the composer.
