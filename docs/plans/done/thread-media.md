# Pictures, GIFs, videos and sounds in a thread's notes (roadmap row 143)

Asked for by the user on 2026-11-08, from Threads' composer, where each
note of a thread can carry pictures, a GIF, a video or a sound. Planned the
same day.

**Status:** done. Phase 1 on 2026-11-08: photos in any note. Phase 2 on
2026-11-09: GIFs, videos and sounds.

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

**Done 2026-11-09,** in one session, as planned, so:

1. **Where the rest waits:** with the file's upload, on the server, so that
   the writer may leave the page once the files are sent, as with a video
   reply. `POST /videos/:owner_id/uploads` takes `thread_after`, the notes
   after it up to the next file, checked as a thread's are when it begins,
   and `after_upload`, the writer's upload before it in the same thread
   (`video_uploads.thread_after` and `after_upload_id`, migration
   `20261122000018_thread_media`).
2. **The worker** makes the file's post, then the notes after it in one go
   (`Medium.AddThread`, `anahita-libs/threads`), then lets the next upload
   go to answer the last of them (`FollowOn`). An upload that waits is not
   taken until then.
3. **A file that fails** stops the thread there: the uploads after it fail
   as `thread_stopped` (`StopThreads`, after each failure and in the hourly
   tidy). Notes after a file that cannot be made, a picture in them gone,
   fail the file too, as `thread_notes`, its post taken back. The app says
   why the first failed.
4. **The app** sends a thread's files one after another, with the + button's
   progress counted over them all, and follows the last. The notes before
   the first file are made at once, as a thread.
5. **A thread's chain** in a list is counted through photos, videos and
   sounds too, not notes alone, which a photo had ended since phase 1.
6. **Not done:** a link card in the notes after a file, which only
   text-service makes; their links are links.
7. Tried in headless Chrome: a post of four (words, a video, words, a
   video) and a reply of three (words, a video, words), made in order and
   counted as threads; a broken first video, which stopped the thread with
   its reason said. Deleted afterwards.

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
