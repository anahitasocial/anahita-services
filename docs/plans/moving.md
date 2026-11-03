# Moving: who you follow, your followers, and your posts (roadmap row 124)

Asked for by the user on 2026-11-03: export and import of who one follows;
moving from one Anahita installation to another, "by initiating a request,
moving their leaders over, and perhaps leaving a redirect mechanism to their
new profile on the old server"; and moving between Anahita and Mastodon or
servers like it. Not built.

## Contents

1. [Why](#why)
2. [What there is now](#what-there-is-now)
3. [How Mastodon does it](#how-mastodon-does-it)
4. [Part 1: who you follow, out and in](#part-1-who-you-follow-out-and-in)
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
   them and whom they blocked, as JSON (`data/following.json`,
   `followers.json`, `blocked.json`), and their posts and pictures
   ([Download your data](../data-export.md)). Nothing reads it back in.
2. **Nothing of moving:** no `alsoKnownAs` on a profile, no `Move` sent or
   heeded, no `movedTo`. When somebody followed from here moves on Mastodon,
   the people here who follow them go on following the old account.

## How Mastodon does it

1. **Export and import,** as CSV: who one follows (one line each,
   `Account address,Show boosts,Notify on new posts,Languages`), lists,
   blocks, mutes, blocked servers and bookmarks. An import is merged with
   what one has, or put in its place.
2. **Moving:** the new account names the old as an alias (`alsoKnownAs`),
   which shows one person holds both. From the old one, the move is started:
   a `Move` is sent to every follower, and their servers have them follow
   the new account instead. The old profile says it has moved, with a link,
   and does nothing more. Followers move; who one follows does not, and is
   brought by importing the CSV. Not again for thirty days.
3. **Posts do not move.** The archive can be downloaded, as ActivityPub's
   `outbox.json`; nothing imports it.

## Part 1: who you follow, out and in

1. **Out, in Mastodon's shape:** `following_accounts.csv`, one line a
   profile by its address (`ana@banu.io`, `alice@mastodon.social`), which
   Mastodon, Pixelfed, GoToSocial and Akkoma read. Groups, hashtags and
   locations followed, which only Anahita has, in a second file,
   `following_anahita.csv`, which they leave be. Blocks the same way,
   `blocked_accounts.csv`.
2. **In, from either:** each address looked up and followed, here or by
   federation; a group with rules asks for them to be agreed first; one that
   cannot be found or followed is listed, not lost. Merged with who one
   follows already, never in place of it.
3. **From the download,** in the settings beside it, and the files are in
   the download too.

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
3. Lists, mutes and bookmarks, until there is a call for them.

## Phases

1. **Who you follow, out and in,** in Mastodon's CSV: one session.
2. **Following somebody who moved:** one session.
3. **Moving, out and in,** with the signpost: three to four sessions.
4. **Bringing your posts,** from Anahita, then perhaps from Mastodon: three
   to four sessions, decided apart.

Four of FediDB's questions ([direct-messages.md](direct-messages.md#part-2-the-feature-list-answered))
are these: moving with followers, accepting followers moving in, exporting
one's data, and importing follows and blocks.
