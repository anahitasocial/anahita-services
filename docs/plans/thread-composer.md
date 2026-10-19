# Writing a thread in one go

*Roadmap row 101, planned 2026-10-19. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

**Status:** parts 1 to 3 done (2026-10-19); parts 4 and 5 next. Branch
`issues/thread-composer` in both repositories, from `issues/network-limits`.

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
3. **No numbering,** here or on other networks: the thread shows itself
   (the user, 2026-10-19).
4. **Up to 10 parts** (`constants.ThreadPartsMax`).
5. **Posts and replies alike:** a thread can begin a post, or answer one.

## What the code does now (found 2026-10-19)

1. **No transaction.** A post is one gRPC call (`Medium.Add`) and one
   INSERT, and announces itself at once, in a goroutine of its own.
2. **Events are handled in any order.** The consumer runs each message in a
   goroutine of its own, so the parts of a thread reach other services in
   no set order.
3. **Bluesky drops a reply whose parent it has not handled yet:**
   `localPosted` finds no record of the parent in `atproto_sent` and sends
   nothing; and when the parent is recorded but not yet written there,
   `Writer.RefOf` answers `ErrNobody`, which is made permanent.
4. **The fediverse catches up late:** a reply whose parent is not yet in
   `activitypub_objects` is passed over, and the sweep sends it within
   about five minutes, parents before children.
5. **Notifications:** every part of a thread under somebody else's post
   would tell the root's author and every subscriber again.

## Design

### 1. One transaction (graph-grpc-service, anahita-libs) — done 2026-10-19

1. `Nodes.AddThread` writes the parts in one transaction, each after the
   first a reply to the one before, placed as `placeInThread` places a
   reply; the first answers `parent`, or is a post.
2. `Medium.AddThread` (`MediumThreadAddRequest`) checks the parent and each
   part as `Add` does, tags the first part's locations, recounts the
   replies, and announces the parts in order once all are made, each with
   its place in the thread (`EventPayload.Part`: 1, 2, …).
3. `MediaGRPC.AddThread` in anahita-libs.
4. Tests: `thread_add_integration_test.go`, a thread in order under one
   root, a thread at the top, and a thread with a part that cannot be made,
   which leaves none.

### 2. The request (text-service) — done 2026-10-19

`POST /notes/:owner_id/thread` for a thread that begins a post, with what a
post takes (access, who may reply, quote policy, locations) and `parts`, a
list of `{ content, language, link_preview_url }`. `POST /notes/:id/replies/thread`
for one that answers a post, with `parts` alone. Each part is checked as a
note or a reply is; up to ten. Notes only at first: pictures in a part are
later.

Built as planned (`thread_add.go`): the first part is checked by the same
code as a single note or reply (`newNote`, `newReply`, taken out of `Add` and
`AddReply`), and the rest are replies owned by the writer, or by a website
in its own thread. The answer is `{ "parts": [ … ] }`. Tried on dev: a thread
of three at the top and one of two under it, read back in order by
`GET /notes/:id/thread`, with the replies counted; eleven parts refused.

### 3. Told once (notification-service) — done 2026-10-19

A part after the first (`Part > 1`) tells nobody: whoever is told of the
thread is told of its first part. A name mentioned in any part is still
told, as it is somebody else.

Built as planned (`event_node_medium.go`), tested in
`thread_told_once_test.go`. Tried on dev: Worf's thread of three under a post
of the test account made one `reply_add`, for its first note.

### 4. In order on other networks (atproto-service, activitypub-service)

1. Bluesky: a reply whose parent is the writer's own post here, not yet
   recorded, records the parent first; and a parent recorded but not yet
   written is a reason to try again shortly, not to give up. The parent's
   reference is read from `atproto_sent` once written there, not from the
   AppView, which may not have it yet.
2. Fediverse: the same for `activitypub_objects`, so the thread goes out
   at once and not at the next sweep.

### 5. The composer (web)

"Add to thread" under the text makes the next part; each part shows what
row 100's lines show; Post sends them all. On a phone, the whole screen.

## Checking

A thread of three with pictures, as a reply and as a post, from a federating
test profile; a failure in the second part leaves nothing posted. What went
out read on the PDS and the outbox, in order.

## Docs to change

`docs/api.md` and `docs/api/text-service.json`, `docs/federation.md`,
anahita-web `docs/` for the composer.
