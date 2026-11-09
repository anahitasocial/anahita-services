# Short posts of 500 characters (roadmap row 144)

Asked for by the user on 2026-11-08: "I think it's time that we lower our
char limit for notes and replies." Planned the same day.

**Status:** done 2026-11-08.

## Contents

1. [Why](#why)
2. [What there is now](#what-there-is-now)
3. [Elsewhere](#elsewhere)
4. [Decisions](#decisions)
5. [Phases](#phases)
6. [Checking](#checking)
7. [Docs to change](#docs-to-change)

## Why

A note here holds 8,000 characters, a number older than this repository's
history and never chosen for it. Long writing has its own kinds of post,
articles and topics; a short post that reads the same here, on Mastodon
and on Threads is 500 characters, and anything longer is a thread.

## What there is now

1. **8,000 everywhere a post has words** (`max=8000`): notes and replies
   (`text-service/requests/note.go`, `reply.go`), a thread's parts
   (`thread_add.go`), a photo post and a photo reply (`photo-service`), a
   video or a sound (`video-service`), articles and topics
   (`anahita-libs/requests/media.go`), and an event's details
   (`actor-service`).
2. **Every character counts,** a link's whole address too, in the
   composer (`utils/threadSplit.js`, `lengthOf`) and on the server.
3. **To Bluesky,** an address standing alone is sent whole
   (`atproto-service/bsky/outgoing.go`), and counts whole against its 300.

## Elsewhere

1. **Mastodon:** 500 by default, some servers more; a link counts as 23,
   however long, and is shown shortened.
2. **Threads:** 500.
3. **Bluesky:** 300, of the words shown; its app shortens a link to a few
   words, with the whole address beside them, as a facet.

## Decisions

Settled with the user on 2026-11-08:

1. **500 characters** for every short post: notes, replies, a thread's
   notes, and the words of a photo, video or audio post. A post that goes
   to Bluesky is still counted against its 300.
2. **Articles, topics and an event's details keep 8,000.**
3. **A link counts as 23,** however long, as on Mastodon; a Markdown link
   `[words](address)` counts its words.
4. **To Bluesky, a link standing alone is shortened** to at most 23
   characters, with the whole address beside it, so it costs there what it
   costs here.
5. **An edit is held to 500 too.** A post that is longer already is
   shortened or left as it is; nothing is converted. (None is expected on
   banu.io; on dev, five replies, two photo posts and one note from the old
   anahita.io, all from 2010 to 2015.)

## Phases

### 1. The count, the limit and Bluesky

1. **One count, both sides.** anahita-libs counts a post's length with
   each link as 23 (`posts.Length`), and a validation rule, `post_length`,
   holds the short posts' words to 500 (`constants.PostMaxLength`). A raw
   cap stays, at 8,000, against anything sent that is not a post's words.
2. **The requests:** every short post's words, when added and when edited.
   Refused with 400 and `content_too_long`, with the most it may have.
3. **NodeInfo** says the limit (`postMaxLength`), for the web app and
   other clients.
4. **Bluesky:** a link standing alone sent as its short form, with its
   address as a facet.
5. **The web app:** `limits.js` and `threadSplit.lengthOf` count as the
   server does, and the split counts the same way; the count says 500
   where a post stays here or goes to the fediverse.

Estimate: 1 to 2 sessions.

**Done 2026-11-08,** as planned, with these besides, at the user's word
the same day:

1. **No refusal code of its own:** a post too long is refused with 400, as
   anything not well formed is; the web app never sends one.
2. **Bluesky is a warning, not a limit:** the count is against 500
   wherever a post goes; where it goes to Bluesky, the count turns to the
   warning's colour past 300, and the line beside it says Bluesky shows the
   first 300 with a link to the rest.
3. **Counted down as Bluesky's app does:** what is left, then a ring that
   fills (`CharacterCount` in the web app's `Composer/ThreadParts.jsx`),
   red and below zero past 500, where Next and Post stop and "Split into a
   thread" is offered.
4. Tried on dev: 501 characters refused, 474 and a link of 324 taken, an
   edit to 501 refused; the count at 120, 354 and 531 at both sizes.

## Checking

Headless Chrome at both sizes: a note of 500 with a long link posts, and
one of 501 is offered a split; an edit past 500 is refused, and Cancel
leaves the post as it was. A note with a long address, sent to a Bluesky
test account, reads its short form, and the link leads to the whole
address. Deleted afterwards.

## Docs to change

1. `docs/api.md`, its table of lengths, and the API reference.
2. `docs/features.md`, "Posts over 500 characters", and FediDB's answer.
3. `docs/federation.md`, what Bluesky is sent.
4. anahita-web `docs/architecture.md`, the composer's count.
5. Both `CHANGELOG.md` files.
