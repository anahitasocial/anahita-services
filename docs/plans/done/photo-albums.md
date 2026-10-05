# Several images per photo post; photo sets removed

*Roadmap item 17 and 18, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
- **Today a photo node holds exactly one file,** in the `nodes` columns `filename`, `filesize` and `mimetype` (`migration-job/schema/init.sql:54-69`).
- **Upload path:** photo-service resizes it in the request and stores each size through file-grpc (`photo-service/cmd/api/handlers/photo.go` ~:300-360, `anahita-libs/utils/images.go`).
- **Limits:** nginx caps a request at **10 MB** (`nginx.conf:8`) and file-grpc messages are limited to **4 MB**, which is fine for one resized image.
- **Legacy photo sets:** the type `node.medium.photo-service.photo-set.v1` appears only in `feed-defaults/defaults.json`. There's no code.

## How many: 4 (decided 2026-09-28)
- **4 images per post,** the same as Mastodon, Bluesky and Pixelfed's default. A constant, `PhotoMaxFiles = 4`, in `anahita-libs/constants`, not an env var.
- It's still published in NodeInfo `metadata` so the web app doesn't hard-code it.
- **Federation (item 8):** posts map one-to-one on both protocols. There's no truncation and no "+N more" link.
- **Confirmed on 2026-10-04:** Bluesky's image schema allows 4 images of at most 2 MB each, and Mastodon's documentation 4 of at most 16 MB. Four stays. The sizes differ, so what is sent to Bluesky has to fit 2 MB: keep a stored size that does, which the resizing here should guarantee.
- **Corrected on 2026-10-05:** Bluesky's 4 is the limit of `app.bsky.embed.images` only. `app.bsky.embed.gallery` takes more (10 in practice). Four stays for now, for Mastodon; raising it is an open note in the federation plan.

## Design: one photo node with ordered attachments (not an album of photo nodes)
- **Why:** a post with slides is *one* thing: one caption, one set of likes and replies, one audience. That's also how ActivityPub (one `Note` with several `attachment`s) and Bluesky (`app.bsky.embed.images`) model it.
- **Why not photo-set nodes:** an album of separate photo nodes means separate likes and replies per image, plus extra joins everywhere.
- **Clean up:** the unused `photo-set` type was removed from `feed-defaults/defaults.json` with item 18.

**Data**
- A side table `photo_files`:
  - `node_id`, `position` (0-based; PK `(node_id, position)`);
  - `filename`, `filesize`, `mimetype`, `width`, `height`;
  - `alt_text varchar(1500)`: alt text for each image, needed for accessibility, and it maps to ActivityPub `name` and Bluesky `alt`.
- **Backward compatible:** keep `nodes.filename` / `filesize` / `mimetype` as a copy of the **first** image. Everything that reads one image today (feeds, cards, OG tags in item 10, search, notifications) keeps working unchanged.
- **Migration:** backfill one `photo_files` row per existing photo (position 0, no alt text), and add the table to `init.sql` and `legacy/upgrade.sql`.

**Upload flow** (each image in its own request, so the 10 MB and 4 MB limits still hold)
1. `POST /photos/uploads` (one image).
   - photo-service validates the type (images only, **re-encoded** as today, item 9's rule), resizes it and stores the sizes under a staging prefix.
   - It returns `{ upload_id, width, height, preview_url }`.
   - Uploads can run in parallel from the browser.
2. `POST /photos/` `{ uploads: [{ upload_id, alt_text }, ...], name, body, access }`.
   - Creates the node, moves the staged files into place in the given order, and writes `photo_files`.
   - Checks: 1 to 4 uploads, all owned by the viewer and unused.
3. Staged uploads not attached within 24 h are deleted by the sweeper.
4. The existing single-file create stays as a shortcut (one image) for older clients.

**Editing:** `PATCH /photos/:id/files` `{ order: [...], alt_texts: {...}, remove: [...], add: [upload_id...] }`.
- Reorder, change alt text, remove, or add up to the limit, all in one request.
- It re-syncs the first-image copy on `nodes`.
- Removing the last image is refused; delete the post instead.

**Reads:** photo responses gain `files: [{ position, urls_by_size, width, height, alt_text }]`, with stable media URLs (item 9 A / item 8 phase 0.4). Feed and list responses include only the first image plus `file_count`, so lists stay light.

**Purge and delete (item 5):** collect every file in `photo_files`, not just `nodes.filename`.

## Web (anahita-web)
- **Composer:**
  - multi-select or drag-and-drop up to the limit, uploading each image as soon as it's picked, with progress;
  - thumbnails you can **drag to reorder**, remove (✕), and give **alt text**;
  - the first image is the cover;
  - then the caption and the item 13 audience picker.
- **Slides:**
  - An MUI carousel in the photo card and on the read page: swipe on touch, arrows and dots, keyboard ←/→, and `alt` on every image.
  - Images load lazily beyond the current one.
  - Lists show the cover with a "1/7" badge.
- **Lightbox:** extend the existing `src/containers/media/Stepper` to step through the files of one post before moving to the next post.
- **Edit dialog:** reorder, alt text, add, remove.
- i18n: "Add alt text", "Photo 2 of 7", and so on.

## Estimate

| Part | Time |
| --- | --- |
| `photo_files` table + backfill migration, staged upload endpoint, multi-file create, edit endpoint, first-image sync, sweeper cleanup, purge update, tests | 6–8 h |
| Web: multi-upload composer with reorder and alt text, carousel on card and read page, Stepper within a post, edit dialog | 6–8 h |
| Constant + NodeInfo field, remove `photo-set` from feed-defaults, docs | 1 h |
| **Total** | **about 12–16 h, 3–4 sessions** |

## Verification
- **Limits:** creating with 5 uploads gives 422, and a 12 MB phone photo uploads fine (each request carries one resized image).
- **Upload validation:** a non-image renamed `.jpg` is refused, and another person's `upload_id` gives 403.
- **Migration:** existing single photos read with `files` of length 1 and look unchanged in feeds.
- **Web:** reordering, then saving, changes the cover shown in feeds. Alt text shows up in the `alt` attribute and in the ActivityPub JSON (item 8). Swipe, arrows and keyboard all work on mobile and desktop.
- **Cleanup:** an upload never attached is gone after 24 h, and purging the owner deletes every file of a 7-image post.

## Decisions (item 17)
- **At most 4 images per post,** matching Mastodon and Bluesky.
- One photo node with ordered attachments and alt text for each image (not albums of photo nodes).
- Federation maps one-to-one on both protocols.

## Dependencies
- **Soft:**
  - item 9 A / item 8 phase 0.4 (stable media URLs for each image);
  - item 13 (audience picker in the composer);
  - item 5 (purge collects all files);
  - item 8 (multi-attachment federation);
  - item 10 (OG uses the first image);
  - item 16 (a later "mixed photos and video" post would reuse this attachment table).

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `architecture.md`**: `photo_files`, staged uploads, and the 4-image limit (and that NodeInfo publishes it).
- **web `architecture.md`**: the multi-image composer and the carousel.
- Item 18 needs no page changes: photo sets were never documented.

## What was built for item 17 (2026-10-05)

Described for people running or building on it in `docs/photos.md`.

**Server.**
- `photo_files` and `photo_uploads` (migration `20261008000001_photo_files.sql`, `init.sql`, `legacy/upgrade.sql`), with one row backfilled for each photo there already was. Owned by graph-grpc, reached through four new calls on its medium service.
- `POST /photos/:owner_id/uploads`; `POST /photos/:owner_id/` takes `uploads`; `PATCH /photos/:id/files`. The one-file form post still works.
- `files` on photo responses from photo-service and feed-service. `photoMaxFiles` in NodeInfo.
- The sweeper removes uploads unused after a day. Deleting a post and purging an account remove every image.
- Preview mode sends a visitor one small image and `files_held_back`.

**Web.** A photo composer that takes several images, with order, removal and a description for each; slides on the post card, in the feed and on the photo's page; the lightbox goes through a post's images; "Edit photos" in a photo's menu.

**Where it differs from the plan.**
- **No staging prefix and no moving of files.** An image is stored under the profile, not the post, so an upload is stored where it will stay and "attaching" is a row changing tables.
- **Editing sends the whole list,** not `order`, `alt_texts`, `remove` and `add`. One rule, and nothing depends on the order separate changes are applied in.
- **No `filesize` column, and no `file_count` in lists.** Feeds carry the images themselves, read with one extra query for the page; other lists show the first image.
- **Arrows, not drag, to reorder.**
- **An image is named by its stored filename** in the API, not by its position, so an edit made from a stale copy cannot move the wrong image.

**Found and fixed on the way.**
- **A purge left photos in storage.** It built each photo's path without the service and the owner that are part of it, asked storage to delete objects that were never there, and got no error. Its test passed because it only looked for the word "photo" in the path. Both are fixed, and the test now names exact paths.
- **`original` and `square` were squeezed.** For a photo posted through these services, `original` was a 320 pixel square and `square` the whole picture pressed into 640 by 640. `original` is what the photo's page and the zoom show. Now `original` is the picture at its own size and `square` is cut from the middle. Photos posted before the move to Go were never affected; photos posted since, and before this, still have the squeezed copies.
- **A file that was not an image left a photo post with no photo,** because the post was made before the file was looked at. The images are now settled first.
- **A photo over 4 MB was refused,** which is most photos straight off a phone. The limit was on the file as it arrived, though only reduced copies are stored. It is now 10 MB, with a limit on pixels checked before the picture is opened.

**Tests.** The rules for which images a post may have, as one function; that a post binds both as JSON with uploads and as a form with a file; the tables against real MySQL, including two posts claiming one image, the sweeper, and the purge with exact paths; the preview; attaching and presigning; the image helpers. The migration was run fresh, as an upgrade (twice) and through the legacy path (1,661 photos, one row each), and the three schemas compared. In the web app, the list logic.

**Not checked.** Uploading and posting through the running stack, which needs a signed-in session: reading was checked on the dev stack, posting was not. Nothing in the web app has been tried in a browser.

**Left for later.** Putting right the squeezed `original` and `square` of photos posted between the move to Go and this. It needs each photo's largest good copy read back from storage and the two written again.

## Photo sets removed (roadmap item 18)

### Decision
**Don't build albums or collections** (decided 2026-09-28).
- **Already covered elsewhere:**
  - organising and showcasing is covered by 4-image posts (item 17), pinned posts (item 15) and hand-picked feeds (item 14);
  - slides already exist within a post (item 17).
- **Collections of other people's posts would need a consent flow,** plus access filtering for each viewer.
- **No fediverse client renders post collections,** so they wouldn't federate usefully.
- **Easy to revisit later:** private bookmarks with folders (the Mastodon and Bluesky model, which needs no consent) would be the natural place to start.

### What exists
- `migration-job/legacy/upgrade.sql` carried legacy albums over:
  - `node.medium.photo-service.photo-set.v1` nodes (`:72`);
  - `edge.photo.photo-set.v1` edges between photos and sets (`:115`, with `node_b_type` at `:140`).
- **No Go code reads or serves them,** and the web app doesn't show them. They're orphaned data.
- `feed-defaults/defaults.json` still lists the `photo-set` type (item 17 already removes it).

### Implementation
1. **Count first** (read-only, on the dev DB and production), and record the counts in the migration comment:
   - photo-set nodes;
   - the `edge.photo.photo-set.v1` edges;
   - comments, likes, subscriptions, hashtag and mention edges, and notifications that point **at a set**.
2. **A goose migration, `…_delete_legacy_photo_sets.sql`,** with the stored-procedure pattern and the same delete order as item 1:
   1. notifications whose `object_id` is a set (or a comment on a set);
   2. edges touching a set or its comments, including every `edge.photo.photo-set.v1`;
   3. comment nodes under sets;
   4. the set nodes.
   - **Photos are never deleted.** They belong to their owner directly, not to the set.
   - It's idempotent, and Down is `SELECT 1` (a deliberate, irreversible deletion).
3. **Legacy installs:** mirror it in `legacy/upgrade.sql`. Simplest: stop converting sets at `:72`, `:115` and `:140` and delete the `com:photos.domain.entity.set` rows and their edges in the same step. Then run `migration-job/check_sql_order.py`.
4. **Set covers:** if a set's cover image points at a file that's also a photo, keep the file. Only delete storage objects that belong to the set node itself, if there are any (check during implementation).
5. Remove the `edge.photo.photo-set.v1` string from constants and docs, if it appears anywhere.

### Estimate
**About 2–3 h, half a session:** counting, the migration, the `upgrade.sql` change and checks.

### Verification
- After the migration, zero `photo-set` nodes and zero `edge.photo.photo-set.v1` edges remain, and the photo count is **unchanged**.
- No edges or notifications point at missing nodes.
- A second run deletes nothing.
- A fresh legacy upgrade on a copy of the legacy dump produces no sets.

### Dependencies
- None. It can run alongside items 1 and 17. Do it before item 5 (the complete purge), so the purge never has to deal with sets.

### What was built (2026-10-03)

- **Migration** `migration-job/migrations/20261003000002_delete_photo_sets.sql`, and the same deletion in `legacy/upgrade.sql`. On the reference database both remove 151 sets, the 52 comments under them, 1,259 edges (1,018 of them set memberships) and 1 notification. The photo count is unchanged, nothing is left pointing at a missing node, and a second run removes nothing.
- **No storage cleanup was needed.** A set's `filename` is its cover photo's name (133 of the 135 that have one match a photo in that set; the other 2 name photos that no longer exist), not a file of its own.
- **`upgrade.sql` still converts the legacy set type and edge type** before deleting them, rather than skipping the conversion as step 3 suggested. That keeps one identifying condition for both paths.
- **`feed-defaults/defaults.json`** no longer lists the `photo-set` type, so feeds stop asking for it.
- **Web:** `photo-sets` is gone from `TAB_COMPONENTS` and from the medium type list.
- **Applied to the dev database** on 2026-10-03.


## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Item 17 (several images per post) | done 2026-10-05, on branch `m4-posting-experience` |
| Item 18 (photo sets removed) | done 2026-10-03, on branch `m1-safety-cleanup` |
