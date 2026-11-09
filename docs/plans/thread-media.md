# Pictures, GIFs, videos and sounds in a thread's notes (roadmap row 143)

Asked for by the user on 2026-11-08, from Threads' composer, where each
note of a thread can carry pictures, a GIF, a video or a sound. Planned the
same day.

**Status:** phase 1 done 2026-11-08: photos in any note. Phase 2, GIFs,
videos and sounds, next.

## Contents

1. [Why](#why)
2. [What there is now](#what-there-is-now)
3. [Elsewhere](#elsewhere)
4. [Decisions](#decisions)
5. [Phases](#phases)
6. [Checking](#checking)
7. [Docs to change](#docs-to-change)

## Why

A thread is how a longer story is told, and a story is told with pictures
too: a trip, an event, the steps of something made. Today a thread is of
words alone (row 101: "pictures in a thread's notes are for later"), so
the composer hides Add to thread as soon as a photo is added.

## What there is now

1. **A thread is made in one go:** `POST /notes/:owner_id/thread` and
   `POST /notes/:id/replies/thread` (text-service, `thread_add.go`) take
   `parts` of `{ content, language, link_preview_url }`; the graph's
   `Medium.AddThread` writes them in one transaction and announces them in
   order once all are there.
2. **A reply can carry pictures:** `POST /photos/:id/replies`
   (photo-service, `photo_reply.go`) makes a photo post in the thread from
   pictures uploaded first, for the person replying; the rules for which
   uploads a post may use are `planFiles` (`photo_plan.go`).
3. **An upload belongs to a profile:** it can be used only by whoever
   uploaded it, on the profile it was uploaded for, where it is stored. A
   thread's first note is on the profile it is posted on; the notes after
   it belong to their writer.
4. **A GIF, a video or a sound is prepared by a worker** (video-service),
   seconds or minutes after it is sent, and joins its thread only then.

## Elsewhere

1. **Mastodon:** each post of a thread may carry up to four pictures, or
   one video or sound. Its own web app has no thread composer: a thread is
   written by replying to one's own post, one at a time (the user, on
   mastodon.world, 2026-11-08); some apps write a thread at once.
2. **Bluesky:** its app writes a thread with pictures or a video in each
   post.
3. **Pixelfed:** a reply is words; one with pictures is taken in over
   ActivityPub, though it may show without them.

A note with pictures in a thread goes out as any reply with pictures does,
so it reads whole on Mastodon and Bluesky.

## Decisions

1. **Photos first, then the rest** (the user, 2026-11-08: "add it to the
   plan and make it happen").
2. **Any note, the first too.** A note with pictures is a photo post in the
   thread, as a reply with pictures is; with words alone, a note.
3. **As many pictures as a post takes** (`PHOTO_MAX_FILES`), with the line
   saying other networks show the first four, as under a post.
4. **Still all or none, in order** for photos, which are uploaded before
   Post is pressed. Videos, GIFs and sounds hold back the notes after them
   until they are ready (phase 2).
5. **Tagging people stays on posts:** a note of a thread is not tagged.

## Phases

### 1. Photos in any note

1. **Shared rules.** `planFiles`, `FileRef` and the refusals move from
   photo-service to anahita-libs, so text-service checks a part's pictures
   by the same rules.
2. **The request.** Each part may have `uploads`, as a photo post's. A part
   with them is a photo post: the first checked as a photo post on the
   profile (and refused where photos are off), the rest as photo replies
   by their writer. Its uploads are for the profile it is on.
3. **The graph.** `MediumThreadAddRequest` gains each part's files.
   `Medium.AddThread` sets them before announcing anything; if any cannot
   be set, every part made is removed, and nothing is announced.
4. **The composer and the reply form.** Each note has its own small Photo
   button and its own strip of pictures; the first note keeps the bottom
   row. The pictures of a note after the first are uploaded for the writer.
   Add to thread stays offered with photos.

Estimate: 2 sessions.

**Done 2026-11-08,** as planned, with these besides:

1. The graph sets a part's images in the thread's own transaction
   (`setPhotoFiles`, shared with `PhotoFiles.Set`), not after it: a picture
   that cannot be set leaves no part, and every upload kept. Tested in
   `TestAThreadsPicturesAreMadeWithIt`.
2. `allow_download` on the thread's request, for the first part's
   pictures; tagging the first part's people, as on a photo post, by the web
   app once the thread is made.
3. In the composer, at the user's word the same day: every note's Photo,
   GIF, Video and Audio icons under its words, not at the dialog's foot;
   the writer's name above each note, with its place in the thread
   ("2/3"); the words at 15 px.
4. Tried in headless Chrome at both sizes: a thread of three, photos in the
   first and the third, made as a photo post, a note and a photo post, in
   order, each picture stored and its upload gone; deleted afterwards.

### 2. GIFs, videos and sounds

1. A thread with one is made up to the first note that has one; that note
   is made when its file is ready, and the notes after it then, in order,
   each a reply to the one before.
2. Its progress is the + button's, for the whole thread; a file that
   cannot be prepared stops the thread there, and its writer is told.
3. The composer: GIF, Video and Audio beside Photo on each note.

Estimate: 2 sessions.

## Checking

Headless Chrome at desktop and mobile sizes: a thread of three with
pictures in the first and the third, as a post and as a reply, read back in
order by `GET /notes/:id/thread`, with each picture stored; a part whose
upload is not the writer's refuses the whole thread, and nothing is left.
What goes out read in the outbox. Deleted afterwards.

## Docs to change

1. `docs/api.md`, `docs/api/text-service.json` and
   `docs/api/photo-service.json`.
2. `docs/federation.md`, where a thread is said to be notes.
3. anahita-web `docs/architecture.md`, a thread written in one go.
4. anahita-web `CHANGELOG.md`.
