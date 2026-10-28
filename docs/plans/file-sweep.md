# Sweeping away stored files nothing names

anahita-web roadmap row 106. Planned 2026-10-28, asked for by the user after
banu.io's files were moved (its row 16).

## Why

A file in the bucket is named by a row: an avatar by its profile's
`avatar_filename`, a photo's sizes by `photo_files`, and so on. When the row
goes, the file should go with it, and mostly does: a purge deletes an
account's files, deleting a post deletes its images, and the sweeper already
removes images uploaded for a post that was never made. But not always. A
purge whose file deletions failed (it carries on without them when
`FILE_SERVICE_ADDR` is unset, or the store did not answer), a replaced avatar
whose old sizes were left, a worker stopped half-way: each leaves files that
nothing names, kept, paid for, and in a person's export of their own data
nowhere. Today nothing finds them.

## What counts as named

Only `assets/v1/`, and only the folders whose keys this knows. Each key says
whose file it is and what it is called, so each is checked against the row
that should name it:

| Folder | Key | Named when |
| --- | --- | --- |
| `identity-image-service/avatars/` | `<person\|group\|…>/<id>/<name>_<size>.<ext>` | the node `<id>` exists and its `avatar_filename` is `<name>.<ext>` |
| `identity-image-service/covers/` | `<person\|group\|article>/<id>/<name>_<size>.<ext>` | the node exists and its `cover_filename` is `<name>.<ext>` |
| `photo-service/photo/` | `<id>/<name>_<size>.<ext>` | `photo_files` has `<name>.<ext>`, or `photo_uploads` does, an upload not yet posted |
| `video-service/videos/` | the video's sizes and poster | its video node exists |
| `video-service/uploads/` | an upload in progress | `video_uploads` has it |
| `link-preview-service/` | a link card's picture | a node's link preview names it |
| `export-service/` | a person's export | left alone: exports expire after a week by their own rule |

A key in none of these shapes is left alone, and counted.

## How

- **Kept unless the sweep is sure.** A file goes only when its key parses
  into one of the shapes above and the row it names is certainly not there:
  never on a failed or slow query.
- **A day old at least,** by the store's own date, so that nothing being
  uploaded, resized or encoded is touched.
- **A new call in file-grpc-service,** `List(prefix, after, limit)`, limited
  to `assets/v1/`, so the sweeper reads the bucket through the service that
  owns it, as it deletes through it now; no keys to the bucket in the
  sweeper.
- **Once a day, not every ten minutes:** listing a whole bucket costs more
  than reading a table. A CronJob of its own, `file-sweeper`, at night,
  running `sweeper -files-only`, under the sweeper's lock.
- **A dry run first.** `-files-only -dry-run` lists and counts what would go,
  and the CronJob runs dry until the operator sets `FILE_SWEEP=on`, so that
  a site sees what it would remove before anything is.
- **Capped,** a few thousand files a night, and logged as counts by folder,
  not names.

## Tests

- Each folder's shapes parse; a key in no shape is kept.
- An avatar its profile names is kept; the old sizes of a replaced one go; a
  purged profile's go; one made an hour ago is kept.
- A photo in `photo_files` or `photo_uploads` is kept; one in neither, a day
  old, goes.
- A query that fails keeps everything in that batch.
- The dry run removes nothing.

## Documentation

- `docs/deploying.md`, "What is in the bucket": what the file sweep removes.
- `docs/account-lifecycle.md`, "The sweeper": the second CronJob,
  `-files-only`, `-dry-run` and `FILE_SWEEP`.
- `docs/configuration.md`: `FILE_SWEEP`.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-28 |
| Implementation | not begun |
