# Refusing servers from a file

anahita-web roadmap row 108. Asked for by the user on 2026-10-27.

## Why

A super administrator refuses a fediverse server one at a time today, in
Site settings under Fediverse (`POST /activitypub/blocks`;
[Servers that are refused](../federation.md#servers-that-are-refused)). A new
site starts with nobody refused, while the fediverse has servers known for
spam and abuse, and communities keep shared lists of them, in hundreds or
thousands. Typing those in one by one is not a thing anybody will do.

## What is read

A CSV file, in either of two shapes:

| Shape | Looks like | From |
| --- | --- | --- |
| Mastodon's | `#domain,#severity,#reject_media,#reject_reports,#public_comment,#obfuscate`, one server a line | Mastodon's own export, and the shared lists made for it |
| One name a line | `spam.example` | Anything else; a first line `domain` is taken as a heading |

- **Mastodon's severity:** `suspend` is a refusal, which is all Anahita has.
  `silence` (hidden, not refused) and `noop` have no match here: they are
  left out and counted, never taken as refusals without being asked.
- **The reason** is `#public_comment` where it is given, and otherwise "From
  `<file name>`, `<date>`", so that the super administrators who come after
  can tell an imported refusal from one typed in.
- **Names** are taken as `POST /activitypub/blocks` takes them, a name, an
  address or a handle; one that is not a name is left out and counted. A name
  obfuscated with `*` (`#obfuscate`) cannot be refused and is left out.
- **Already refused:** left as it was, its reason kept.
- **At most** 10,000 lines and 1 MB a file.

## How

- **`POST /activitypub/blocks/import`**, the file as `text/csv` or in a form,
  super administrators only and with a step-up, as the single refusal is.
- **Looked at before it is done.** Without `"apply": true` it changes nothing
  and answers what it would do: how many are new, already refused, left out
  for their severity, and not names, with a few of each.
  With it, it refuses the new ones in one transaction and answers the same
  counts.
- **`GET /activitypub/blocks/export`**: the list as Mastodon's CSV, every
  refusal `suspend` with its reason as `#public_comment`, so that a list made
  here can be shared or moved.
- **Nothing is sent** to the servers refused, as now; a refusal takes hold in
  every copy of the service within half a minute.

## In the web app

In Site settings, under Fediverse, beside the dialog to refuse one server:

- **Import:** choose a file; a dialog says what it would do ("312 to refuse,
  40 already refused, 18 left out as silence, 2 not names"), with a few of
  each, then Cancel or **Refuse 312**.
- **Download:** the list as a CSV.

## Tests

- Both shapes read; a heading taken as one; blank lines and `#` comments
  skipped.
- `suspend` refused; `silence` and `noop` left out and counted; an obfuscated
  name left out.
- Looking changes nothing; applying refuses the new ones and leaves the
  existing ones' reasons.
- Too many lines, or too large, refused whole with a plain answer.
- The export read back by the import changes nothing.
- Not a super administrator: `403`; no step-up: `403 step_up_required`.

## Documentation

- `docs/federation.md`, "Servers that are refused": the import, the two
  shapes, what is left out and why, and the export.
- The API reference, from the code.
- anahita-web's `docs/architecture.md`: the two buttons in Site settings.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-27 |
| Implementation | not begun |
