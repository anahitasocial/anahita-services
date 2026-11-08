# Bring your lists in

Somebody arriving from Mastodon, Misskey or another Anahita brings who they
follow, block and mute, and what they saved, in a file: another Anahita's
download, or a single CSV of Mastodon's or Misskey's. The file is read at
once and what is in it shown; nothing is done until they start it; then a
worker goes through it a line at a time, doing each as they would by hand.
Roadmap row 124, part 1 ([Moving](plans/moving.md)).

Its routes are in the API reference: [import-export-service](api/index.html#import-export-service).

## Contents

1. [What a file may be](#what-a-file-may-be)
2. [How each line is done](#how-each-line-is-done)
3. [What could not be done](#what-could-not-be-done)
4. [Limits](#limits)
5. [Endpoints](#endpoints)
6. [The parts](#the-parts)
7. [Where the code is](#where-the-code-is)
8. [What is not built](#what-is-not-built)

## What a file may be

| File | Read as |
| --- | --- |
| Another Anahita's download, the zip of [Download your data](data-export.md) | Its `data/following.json`, `blocked.json`, `muted.json` and `saved.json`, which say the most: a mute's length and whether it hides notifications, and the groups and events followed. Its `mastodon/` folder is read only when there is no `data/` |
| `following_accounts.csv` | Mastodon's: `Account address,Show boosts,Notify on new posts,Languages`. People to follow |
| `blocked_accounts.csv` | One address a line, no header. People to block |
| `muted_accounts.csv` | `Account address,Hide notifications`. People to mute |
| `bookmarks.csv` | One post's address a line, no header, or under `#uri`. Posts to save |
| Misskey's, or any list | One address a line, with or without its first `@`. Its kind by its name, or as the person says (`kind`) |

A CSV is known as Mastodon knows one: by its first header, or, with none,
by its name. One whose kind cannot be told is refused with `kind_unknown`,
and the person is asked which it is. Lists (`List name,Account address`)
and blocked servers (`#domain`) are refused with `not_supported`: Anahita
has neither yet. A list of somebody's followers is not read: their
followers come by moving ([Moving](plans/moving.md), part 3).

## How each line is done

As the person, by the rules the same thing has when done by hand
(`anahita-libs/permissions`):

1. **Who or what is found first.** Somebody here by their username, from a
   handle on this site's domain; somebody elsewhere by their handle, found
   on their own server and kept, by activitypub-service, or by
   atproto-service for a Bluesky handle; a post by its ActivityPub id or
   its `at://` address, the same way. Those two are asked on their internal
   port ([Internal calls](configuration.md#internal-calls)).
2. **Follow:** followed, or asked to be followed when they approve their
   followers; a group or an event whose rules are to be agreed first is not
   followed, and is listed. Nobody is followed across a block.
3. **Block:** not twice, and not somebody who blocks the person already.
4. **Mute:** a person only, not across a block, with what the file said of
   its notifications and its end. One that has ended is not made.
5. **Save:** a post the person may see.
6. **Nothing is undone.** An import adds: it unfollows, unblocks, unmutes
   and unsaves nothing. What the person has already is counted as such
   (`already`), so importing a file twice does nothing twice.

What a line does is told as it would be by hand: a follow to the person's
server, a block to the fediverse and to Bluesky for somebody connected. A
mute and a save are told to nobody.

## What could not be done

Each line ends `done`, `already`, `skipped` (a kind not chosen, or the import
given up) or `failed`, with why in a word, listed in a CSV the person can
download (`GET /imports/:id/failed`):

| Reason | Means |
| --- | --- |
| `not_found` | Nobody, or nothing, at that address |
| `not_on_fediverse` | A group or an event on another Anahita: groups are not on the fediverse, and it is followed by hand there |
| `refused` | On a server, or an account, this site refuses |
| `not_federating` | This site does not take part in that network |
| `unreachable` | Their server could not be reached: importing the file again later may do |
| `blocked` | One of them blocks the other |
| `not_allowed` | Not theirs to follow, or to see |
| `rules_not_agreed` | A group or an event whose rules are to be agreed first |
| `ended` | A mute that ended since the file was made |
| `failed` | Something here went wrong |

## Limits

1. **Mastodon's own:** at most 20,000 lines and 20 MB a file.
2. **One import at a time** for each person, waiting or running: a rule of
   the database (`data_imports.active_person_id`).
3. **A line a second,** whoever's: several people's imports go forward
   together, a line each in turn. 20,000 lines take about five and a half
   hours.
4. **A line left by a worker that stopped** is taken again ten minutes on.

## Endpoints

| Request | Does |
| --- | --- |
| `GET /imports` | The viewer's latest five imports, newest first |
| `POST /imports` (form: `file`, and `kind` for a CSV with no header) | Reads the file and keeps it, `ready`, with how many of each kind |
| `GET /imports/:id` | How it is getting on: for each kind, how many in all, done, already, failed, skipped and left |
| `POST /imports/:id/start` `{ "kinds": ["follow", "block", "mute", "save"] }` | Starts it with the kinds chosen; the others are skipped |
| `GET /imports/:id/failed` | What could not be done, as a CSV |
| `DELETE /imports/:id` | Gives it up: what it has not done is not done |

Every route is the viewer's own; reading a file asks for the
`socialgraph:write` scope. The gateway lets a request to `/imports` be as
large as 21 MB.

## The parts

| Part | Does |
| --- | --- |
| `import-export-service` (`/imports`) | Reads a file and keeps its lines; says how an import is getting on |
| `import-export-worker` | Goes through the started imports, a line a second, beside the downloads it makes |
| `activitypub-service`, `atproto-service` (port 8443) | Find somebody or a post elsewhere and keep it, for the worker |
| `graph-grpc-service` | Follows, blocks, mutes and saves, as for a person's own request |

## Where the code is

| What | Where |
| --- | --- |
| Reading a file | `import-export-service/importfile/importfile.go` (`Read`), tested in `importfile_test.go` |
| The imports and their lines | `import-export-service/data/repositories/imports.go`, tables `data_imports` and `data_import_lines` in `migration-job/schema/init.sql` |
| The endpoints | `import-export-service/cmd/api/handlers/imports.go` |
| Going through the lines | `import-export-service/worker/importer.go` (`Importer`), the acts in `worker/acts.go` (`GraphActs`) |
| Finding somebody elsewhere | `anahita-libs/servicecalls`, `activitypub-service/cmd/api/handlers/internal.go`, `atproto-service/cmd/api/handlers/internal.go` |

## What is not built

1. **Lists, one's own blocked servers, filters,** Misskey's antennas: the
   features are not here yet.
2. **Replacing** one's lists with a file's, as Mastodon's "overwrite" does.
3. **Posts,** between two Anahitas ([Moving](plans/moving.md), part 4).
