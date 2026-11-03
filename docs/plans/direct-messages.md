# Direct messages, and being found on the fediverse (roadmap row 123)

Asked for by the user on 2026-11-03: "Considering that there are many
Mastodon instances out there, shall we build a direct messaging feature?",
after asking whether Anahita can use Sup (https://getsup.net), a messenger
for the fediverse from the makers of Pixelfed. Its maker answered the user's
question from banu.io the same day: "It checks the server on FediDB first,
does your software have nodeinfo support? If not, that is why." Then the
user: "we need to list Anahita on FediDB and also I like to run a check with
you against their feature matrix." Not built.

## Contents

1. [Why](#why)
2. [What there is now](#what-there-is-now)
3. [Part 1: found on FediDB](#part-1-found-on-fedidb)
4. [Part 2: the feature list, answered](#part-2-the-feature-list-answered)
5. [Part 3: messages](#part-3-messages)
6. [Part 4: Sup and Bluesky's chats](#part-4-sup-and-blueskys-chats)
7. [Out of scope](#out-of-scope)
8. [Phases](#phases)

## Why

The fediverse is thousands of Mastodon servers and others like them. A
person there who sends somebody on Anahita a private message today is not
heard: Anahita keeps a post from another server only when it is addressed to
everybody ([Federation](../federation.md#posts-coming-in)), so the message is
dropped, and neither of them is told. And software that lists the
fediverse, as FediDB does, and the apps that read those lists, as Sup does,
do not know Anahita is there.

## What there is now

1. **NodeInfo is served.** `https://banu.io/.well-known/nodeinfo` leads to
   `https://auth.banu.io/nodeinfo/2.1`, which says `anahita`, its version,
   `activitypub`, its users and posts, and what the installation allows
   ([Registration](../registration.md#nodeinfo)). Checked 2026-11-03.
2. **FediDB knows neither.** `api.fedidb.org/v1/servers/banu.io` and
   `/v1/software/anahita` answered 404 on 2026-11-03. FediDB has moved to
   https://fedidb.com, which takes servers to track at `/servers/submit`.
3. **Anahita is compatible with Mastodon in federation**, and not with its
   apps: it does not offer Mastodon's client API (`/api/v1/…`), which
   GoToSocial, Pixelfed and Akkoma do. Decided against for Anahita's own
   clients on 2026-10-12 ([cli-client.md](cli-client.md)).
4. **There are no messages.** Nor a post seen only by those it mentions.

## Part 1: found on FediDB

1. **Submit banu.io** at https://fedidb.com/servers/submit, the user's to
   do, and check what its NodeInfo explorer (`/tools/nodeinfo`) reads of it.
2. **Anahita as a software of its own.** FediDB lists servers under the
   software their NodeInfo names. Find out from FediDB how a new one is
   added (its data sources page, `/resources/data-sources`, or its makers),
   with its name, its site and its repository.
3. **Each installation from then on** is found by its own NodeInfo; nothing
   here changes for it. [Setting up](../setting-up.md) says how to submit
   one.
4. **Then ask Sup again,** once its own check of the server passes.

## Part 2: the feature list, answered

FediDB compares software by 128 questions, in eleven groups: accounts and
profiles, posting, media, video and audio, interactions, privacy and
safety, moderation, discovery, federation and protocols, apps and the API,
and running a server (https://fedidb.com/software/features). The user and
Claude answer each for Anahita, from the code and not from memory:

1. **A page of the docs, `docs/features.md`,** with every question, yes, no
   or in part, and the page that says so. It is what FediDB is told, and
   what an operator choosing software reads; kept up to date as the
   roadmap's rows are done.
2. **Each "no" weighed:** a row of the roadmap when it is wanted, or said to
   be left out on purpose, with why: no profile analytics, no ranked feed
   for everybody, Mastodon's client API, nomadic identity.
3. **A question FediDB does not ask** about something only Anahita has, as
   groups, events, custom feeds or citations, suggested to it, as anybody
   may.

## Part 3: messages

**Between people here, and with the fediverse from the start.**

1. **A conversation** is between two people, or a few, each a node and each
   message a post in it seen only by those in it; or a private mention, as
   the fediverse writes one. Which, settled when it is designed.
2. **Consent first.** Who may message somebody is their choice: anyone,
   people they follow, people who follow them, or nobody. A message from
   anybody else waits as a request, which they accept or decline, as Bluesky
   and Instagram have it. Blocks apply either way.
3. **Said plainly:** messages are not encrypted end to end, so the
   installation's administrators, and another server's for one sent there,
   can read them. The screen says so where a message is written.
4. **Federation.** A message to somebody on another server goes as a
   private mention: a note addressed only to the people it names, which
   Mastodon and nearly every server show as a direct message. One that
   comes in that way lands as a message, where now it is dropped.
5. **Safety.** A message can be reported; what an administrator sees of a
   conversation, and when, is written down before it is built.
6. **Kept.** Deleting a message deletes it for everybody, and is sent on;
   messages are in the download of somebody's data.
7. **Told.** A new message is a notification of its own, grouped by
   conversation, and never emailed.

## Part 4: Sup and Bluesky's chats

1. **Sup,** due in 2027, signs in through a server's Mastodon client API,
   and sends either through the server, as a private mention, or past it,
   between two people who both use Sup. With Part 3, a Sup user anywhere
   can message somebody here. Somebody here signing in to Sup themselves
   would need part of Mastodon's client API, or Sup supporting Anahita:
   decided once Sup is out and says what it needs.
2. **Bluesky's chats** are not in the AT Protocol: they are a service
   Bluesky runs. For somebody who has connected Bluesky, a later row.

## Out of scope

1. Encryption end to end. The fediverse's work on it (MLS) is looked at
   again when it is used by servers.
2. Mastodon's client API as a whole.
3. Calls, voice and video (roadmap row 69).

## Phases

1. **Found:** banu.io submitted, Anahita known to FediDB as a software,
   `setting-up.md` saying how. Mostly the user's, and an hour of Claude's.
2. **The feature list:** `docs/features.md`, the 128 answered, each "no"
   weighed. A session. **Done 2026-11-03:** [Features](../features.md), and
   the answers entered on FediDB.
3. **Messages here,** with consent, requests, reporting and notifications.
   Four to five sessions.
4. **Messages with the fediverse,** private mentions in and out. Two
   sessions.
5. **Sup and Bluesky's chats,** each decided when its time comes.
