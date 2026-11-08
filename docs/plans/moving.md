# Moving: who you follow, your followers, and your posts (roadmap row 124)

Asked for by the user on 2026-11-03: export and import of who one follows;
moving from one Anahita installation to another, "by initiating a request,
moving their leaders over, and perhaps leaving a redirect mechanism to their
new profile on the old server"; and moving between Anahita and Mastodon or
servers like it. Brought forward on 2026-11-08, at the user's word ("focus
on helping people moving in and out"), before the WordPress import, with
the lists that Mastodon, Misskey and Pixelfed move between them. Not built.

## Contents

1. [Why](#why)
2. [What there is now](#what-there-is-now)
3. [How others do it](#how-others-do-it)
4. [Part 1: your lists, out and in](#part-1-your-lists-out-and-in)
5. [Part 2: following somebody who moved](#part-2-following-somebody-who-moved)
6. [Part 3: moving yourself](#part-3-moving-yourself)
7. [Part 4: bringing your posts](#part-4-bringing-your-posts)
8. [Bluesky](#bluesky)
9. [Out of scope](#out-of-scope)
10. [Phases](#phases)

## Why

Anahita is software a community runs for itself, and sovereignty comes from
being able to leave. A person who moves from one installation to another,
or between Anahita and Mastodon, should take the people they follow and the
people who follow them, and leave a signpost behind, as the fediverse lets
them on every server that does it Mastodon's way.

## What there is now

1. **The download of somebody's data** has who they follow, who follows
   them, whom they blocked and muted, and what they saved, as JSON
   (`data/following.json`, `followers.json`, `blocked.json`, `muted.json`,
   `saved.json`), and their posts and pictures
   ([Download your data](../data-export.md)). Nothing reads it back in.
   (`blocked.json` was empty until 2026-11-08, from the same fault as a
   person's own Blocked list.)
2. **Nothing of moving:** no `alsoKnownAs` on a profile, no `Move` sent or
   heeded, no `movedTo`. When somebody followed from here moves on Mastodon,
   the people here who follow them go on following the old account.

## How others do it

What Mastodon, Misskey and Pixelfed, the largest on the fediverse, let a
person take out and bring in, looked up on 2026-11-08 (Mastodon from its
code, `app/models/export.rb` and `app/models/form/import.rb`):

| Kind | Mastodon | Misskey | Pixelfed | Anahita |
| --- | --- | --- | --- | --- |
| Who one follows | CSV out and in: `Account address,Show boosts,Notify on new posts,Languages` | CSV out and in, one address a line | JSON out | Part 1 |
| Blocks | CSV out and in, one address a line, no header | out and in | JSON out | Part 1 |
| Mutes | CSV out and in: `Account address,Hide notifications` | out and in | JSON out | Part 1 |
| Saved posts (bookmarks) | CSV out and in, one post's address a line, no header | favorites and clips, out | none | Part 1 |
| Lists | CSV out and in: `List name,Account address` | out and in | none | no lists yet |
| Blocked servers, one's own | CSV out and in, `#domain` | none | none | no such setting yet |
| Filters | JSON out | none | none | no filters yet |
| Antennas | none | out and in | none | none |
| Followers | none | none | JSON out | they move by `Move` |
| Moving with followers | `Move` out and in | `Move` out and in | `Move` out | Part 3 |

1. **Mastodon reads a file with no header row** as a list of addresses (or
   of domains, or of posts), which is how Misskey's files, and anybody's
   list typed by hand, are read too. A known first header says which kind a
   file is.
2. **An import is merged with what one has, or put in its place;** at most
   20,000 rows and 20 MB a file.
3. **Moving:** the new account names the old as an alias (`alsoKnownAs`),
   which shows one person holds both. From the old one, the move is started:
   a `Move` is sent to every follower, and their servers have them follow
   the new account instead. The old profile says it has moved, with a link,
   and does nothing more. Followers move; who one follows does not, and is
   brought by the files. Not again for thirty days.
4. **Posts do not move.** Mastodon's archive is ActivityPub's
   `outbox.json`; nothing imports it.

## Part 1: your lists, out and in

Who one follows, blocks and mutes, and what one saved: the four kinds
Anahita has of the lists the others move. Lists, one's own blocked servers
and filters wait for those features (Out of scope).

**Between two Anahitas, the download's JSON** (the user, 2026-11-08): a
download from one Anahita, as a zip, is read whole by another. Its
`data/following.json`, `blocked.json`, `muted.json` and `saved.json` carry
more than Mastodon's files can: groups and events followed, people on
Bluesky, a mute's length and whether it hides notifications. Each entry
gains what another server needs to find it again (built 2026-11-08):

| Field | Holds |
| --- | --- |
| `address` | A person's handle, `ana@banu.io`, `alice@mastodon.social` or `alice.bsky.social` |
| `uri` | Its ActivityPub id, for a profile or a saved post on the fediverse |
| `hide_notifications`, `ends_at` | In `muted.json`: whether its notifications were hidden, and when the mute ends, if it does |

and the download says which version of these files it is (`data/profile.json`,
`format`, 2), so that a later Anahita reads an earlier one's.

Following a hashtag is not an entry here: it is a custom feed with the
hashtag in it, which is one of a person's posts (`posts.ndjson`), and comes
with them in part 4.

**To and from everybody else, Mastodon's files,** in the same download, in
a folder beside the JSON, `mastodon/`, which Mastodon, Misskey, GoToSocial,
Akkoma and the rest read:

| File | Holds |
| --- | --- |
| `following_accounts.csv` | People followed: `Account address,Show boosts,Notify on new posts,Languages`; true, false, and nothing for the languages |
| `blocked_accounts.csv` | People blocked, one address a line |
| `muted_accounts.csv` | People muted: `Account address,Hide notifications`; a mute that ends is written as one that does not |
| `bookmarks.csv` | Saved posts, one post's address a line (its `uri` from elsewhere, its own address here) |

**In,** under Settings, beside the download: one file, either a whole
download (from this Anahita or another) or a single CSV from anywhere.

1. **What it is** is read from it: a download from Anahita by its
   `data/` files, which are used, and not its `mastodon/` ones; a single CSV
   by its first header, and one with none (one address or address of a post
   a line) by what its lines are, or by the person's choice when that
   cannot be told.
2. **Looked at first:** "272 people you follow, 3 you muted, 41 saved
   posts", with a tick for each kind, all ticked, and nothing done until
   Import is pressed.
3. **Merged, never put in place of** what one has: nothing is unfollowed,
   unblocked or unsaved by an import.
4. **Each line, the way a person's own would:** an address looked up, here
   or by federation (WebFinger), and followed, blocked, muted, or its post
   fetched and saved, by the same rules as by hand: a group with rules asks
   for them to be agreed first; a profile that approves its followers is
   asked; one blocked either way is not followed. A group or an event on the old Anahita
   cannot be followed from the new one, as groups are not on the fediverse:
   it is listed, with its address, among what was not done.
5. **In the background,** as a job in import-export-service (row 142, phase 1,
   which this is the first use of): its progress shown while it runs,
   resumed after a restart, and at the end what could not be found or done,
   each with why, to download. Importing the same file again brings nothing
   twice.
6. **Limits,** as Mastodon's: 20,000 lines and 20 MB a file, one import a
   person at a time, and the rate at which it looks people up kept low, so
   that a large list never slows the site or troubles other servers.

## Part 2: following somebody who moved

A `Move` from somebody followed from here, to an account that names them as
`alsoKnownAs`, has everybody here who follows them follow the new account,
and stop following the old, as Mastodon does. Each is told. Needed whatever
is decided about moving oneself, since it happens on other servers now.

## Part 3: moving yourself

**Out, to another Anahita or to Mastodon:**

1. **On the new account,** the old one is named as an alias. Anahita offers
   it in a person's settings, and so does Mastodon.
2. **On the old one,** the move is started, after proving who one is again
   (step-up), with the new address and a sentence of what will happen:
   followers move, posts stay, and the profile here becomes a signpost.
3. **Followers move:** a `Move` to every follower elsewhere; those on this
   installation are moved by it at once.
4. **Who one followed** is brought by Part 1: the new Anahita can fetch the
   old one's export for the person, with their leave; Mastodon imports the
   CSV.
5. **The signpost:** the old profile says "Now at @ana@new.example", its
   handle is still found, `movedTo` is in what other servers read, the web
   app sends a visitor on, and it posts nothing more. Not again for thirty
   days; undone by moving back.

**In, from another Anahita or from Mastodon:** a person here names their
old account as an alias, and starts the move there; their followers arrive
by the `Move`, and who they followed by importing the CSV.

**Who may move:** a person whose profile is on the fediverse, which they
turn on themselves ([Federation](../federation.md#who-is-on-the-fediverse));
the screen says so first. Groups, events and websites are the
installation's and their admins', and do not move.

## Part 4: bringing your posts

Through import-export-service's imports ([imports.md](imports.md)), with the powers it
keeps: a post's own date, and nothing sent for it.

Between two Anahitas, which read each other's download: the new one imports
a person's notes, articles, topics, photos and videos as theirs, with their
dates, audiences and pictures. To the rest of the fediverse they are new
posts, as any import is. What points at the old installation (replies,
mentions, likes) does not come; what to do with it is decided when this is
designed. From Mastodon's `outbox.json` the same, later, and decided apart.

## Bluesky

A person's posts on Bluesky are written to their own Bluesky account, not
to Anahita. Moving, they connect the same Bluesky account on the new
installation, and nothing there changes.

## Out of scope

1. Moving a group, an event or a website.
2. Moving posts to Mastodon, which imports none.
3. Lists, one's own blocked servers, filters and Misskey's antennas: each a
   feature Anahita does not have yet, and a row of its own if asked for;
   their files are then added to Part 1.
4. Replacing one's lists with a file's (Mastodon's "overwrite"): an import
   adds; what one no longer wants is removed by hand.

## Phases

1. **Your lists, out and in:** import-export-service's imports (row 142, phase 1,
   without what only posts need), the files in the download, the import
   with its preview: about two sessions. **The services' part built
   2026-11-08** ([Bring your lists in](../imports.md)); the web app's next.
2. **Following somebody who moved:** one session.
3. **Moving, out and in,** with the signpost: three to four sessions.
4. **Bringing your posts,** from Anahita, then perhaps from Mastodon: three
   to four sessions, decided apart.

Four of FediDB's questions ([direct-messages.md](direct-messages.md#part-2-the-feature-list-answered))
are these: moving with followers, accepting followers moving in, exporting
one's data, and importing follows and blocks.
