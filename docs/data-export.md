# Download your data

Anybody can have a copy of what they have put here: everything they wrote,
the pictures and videos they posted, who they follow and who follows them.
They ask for it in their settings; a worker gathers it into one zip file;
they fetch it within a week.

Its routes are in the API reference: [export-service](api/index.html#export-service).

## Contents

- [The parts](#the-parts)
- [Asking for one](#asking-for-one)
- [What is in an archive](#what-is-in-an-archive)
- [What is not](#what-is-not)
- [How it is made](#how-it-is-made)
- [Keeping and deleting](#keeping-and-deleting)
- [Endpoints](#endpoints)
- [Settings](#settings)
- [Running it, and not running it](#running-it-and-not-running-it)
- [What is not built](#what-is-not-built)

## The parts

| Part | What it does |
| --- | --- |
| `export-service` (port 8104, `/exports`) | Takes a request, says how it is getting on, and gives the address the archive is fetched from |
| `export-worker` | Makes each archive, stores it, and deletes the ones that have been kept their time. Answers no requests |
| `data_exports` | One row for each request: the request, the work on it, and the record of it |
| `graph-grpc-service` | `ListAuthored`, the list of everything one person wrote |
| `file-grpc-service` | Keeps archives under `assets/v1/export-service/`, and only ever sends one as a file to save |

One module, `export-service`, with two programs and two images, as the
video service is.

## Asking for one

`POST /exports`, by somebody who has **just proved who they are** at
auth-service's `/reauth/` endpoints, the same proof deleting a profile
asks for. An archive holds what only its owner may see, and a session left
open on a shared computer must not be enough to take it. One proof makes
one export. Where the proof cannot be checked, because Redis is not
reachable, the request is refused.

| Refused with | When |
| --- | --- |
| `403 step_up_required` | No fresh proof |
| `409 active` | One is already waiting or being made. One at a time for each person, which is a rule of the table (a unique index) and not of whichever request arrived first |
| `429 too_soon`, with `next_at` | The last one was asked for less than `EXPORT_EVERY_DAYS` ago. One that failed or was given up does not count |

Where the request came from, its address and browser, is kept on the row.

## What is in an archive

| File | What |
| --- | --- |
| `index.html` | Everything here as a page to read. It has its own styles, no scripts, and fetches nothing: it works with no connection |
| `README.txt` | What each file is, and how many posts and files there are |
| `data/profile.json` | The profile |
| `data/posts.ndjson` | Everything written, one post on each line: notes, articles, topics, photos, videos, replies, reposts and custom feeds, wherever each was posted, **including what only its author may see** |
| `data/following.json`, `followers.json`, `blocked.json` | Who they follow, who follows them, who they blocked |
| `data/liked.json`, `saved.json` | What they liked and saved, by name and address |
| `actor.json`, `outbox.json` | The profile and the posts as ActivityStreams, the form other fediverse software reads. What was not public is addressed to nobody |
| `media/<post id>/` | Each post's pictures at the largest size kept, a video at its largest size, an article's cover |
| `media/profile/` | The avatar and the cover |

Every name and every post is somebody's text, and is written into the page
as text: the page is made with a template that escapes all of it.

## What is not

- **What other people wrote.** A reply names the post it answers and a
  repost the post it reposts; neither holds it.
- **What was removed.** It is gone for its author too.
- **The account**: the email address, sign-ins, passkeys. Those are
  auth-service's, which this does not read.
- **Files storage does not have.** A post naming a picture that is not
  there keeps its text; the file is left out and counted in `README.txt`,
  so one missing picture does not cost somebody their archive.

## How it is made

1. A worker takes the oldest request waiting (`SELECT … FOR UPDATE SKIP
   LOCKED`, so several workers each take a different one).
2. It reads the person's data through the graph, as that person, a page of
   100 at a time, and writes the archive **to a file of its own as it
   goes**. Nothing is held whole in memory: an archive is as large as
   everything somebody posted.
3. Pictures and videos are copied in from storage as they are, not
   compressed again.
4. The file is stored under `assets/v1/export-service/<person id>/<export id>.zip`, and the
   request is marked `ready`.

Between each page and each file the worker asks whether the request has
been given up, and stops if it has. One given up in the moment it was
finished is deleted and not marked ready.

A worker that dies leaves its request taken; another takes it after two
hours. A request is tried three times, then marked `failed`.

Nothing that is in an archive is logged: only that one was made, how big
it is and how many posts and files it holds.

On the development data, an account with 8,410 posts and 840 MB of
pictures took five and a half minutes.

## Keeping and deleting

An archive is kept for `EXPORT_KEEP_DAYS`, a week unless said, and then
deleted from storage by the worker, the file first and the row marked
`expired` after. Its owner can delete it sooner.

It is fetched from an address signed for a few minutes, made afresh each
time the export is read, which sends it as a file to save. `file-grpc-service`
sends anything under that folder that way and no other, whatever is in it.

## Endpoints

All of them are the viewer's own: an export belongs to whoever asked for
it, and anybody else is told there is none.

| Method and path | What |
| --- | --- |
| `POST /exports` | Ask for one |
| `GET /exports` | The latest ten, with `next_at` when it is too soon to ask again, `every_days` and `keep_days` |
| `GET /exports/:id` | One, with `download_url` while it is ready |
| `DELETE /exports/:id` | Give it up while it is being made, or delete its archive |

`status` is `pending`, `building`, `ready`, `failed`, `cancelled` or
`expired`.

## Settings

| Variable | Value |
| --- | --- |
| `EXPORT_EVERY_DAYS` | Days somebody waits between one export and the next. `7` unless said; `0` for no wait |
| `EXPORT_KEEP_DAYS` | Days an archive is kept. `7` unless said |
| `PUBLIC_WEB_URL` | Worker: the site's address, for the links in an archive back to where each thing is |
| `SITE_NAME` | Worker: the site's name, for the page's heading. `Anahita` unless said |
| `EXPORT_WORK_DIR` | Worker: where an archive is made before it is stored. `/work` in the image. Needs room for the largest one |

## Running it, and not running it

`make export` builds and starts both containers. They need the database,
Redis, `graph-grpc-service` and `file-grpc-service`.

Both are optional. Without them the web app's setting says the download is
not available here.

## What is not built

- Telling the person, by a notification or an email, that their archive is
  ready. The settings page says.
- The account's own data, from auth-service.
- A Bluesky repository file, which comes with federation.
- Deleting somebody's archives when their account is purged. They are
  deleted when their week is up.
