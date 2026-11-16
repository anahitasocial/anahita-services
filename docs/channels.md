# Channels

A channel is a profile for an organization, a brand or a project that only
it posts on. Its admins write as it: the channel is the author shown of
everything on it, and its followers reply, like, repost and quote, and post
nothing on it. A channel may be public or private as a group may, host
events as a group does, and read a website's feed: an RSS, Atom or JSON feed
on a domain its admins prove is theirs, each new item of which becomes one
of its notes. What every profile has, a channel has too
([Profiles](profiles.md)); this page is what only a channel has. In the API
it is under `/channels/`, sent as `actor.ChannelNode`; its feed is
syndication-service's.

Channels were websites until roadmap row 147 (2026-11-16): a website's feed
as a profile, made by connecting the feed. Why they are built this way is in
the plans, [plans/done/channels.md](plans/done/channels.md) and, for the feed,
[plans/done/website.md](plans/done/website.md).

## Contents

1. [Who does what](#who-does-what)
2. [Making a channel](#making-a-channel)
3. [What it posts](#what-it-posts)
4. [Who wrote it](#who-wrote-it)
5. [Replies as the channel](#replies-as-the-channel)
6. [Privacy](#privacy)
7. [Events](#events)
8. [Its feed](#its-feed)
9. [Proving a domain](#proving-a-domain)
10. [Reading the feed](#reading-the-feed)
11. [The feed's notes](#the-feeds-notes)
12. [On the fediverse](#on-the-fediverse)
13. [On Bluesky](#on-bluesky)
14. [WebSub](#websub)
15. [Pausing and starting again](#pausing-and-starting-again)
16. [Settings](#settings)
17. [The tables](#the-tables)
18. [Endpoints](#endpoints)
19. [What is not built](#what-is-not-built)
20. [Where the code is](#where-the-code-is)

## Who does what

| Piece | Does |
| --- | --- |
| `actor-service` | Serves the channel as a profile under `/channels/`, as it serves groups: made, read, changed, its privacy, admins, features, events, archived, deleted |
| `text-service`, `photo-service`, `video-service` | Make what its admins post on it the channel's own; under its posts, make a reply by one of its admins the channel's |
| `syndication-service` | Its feed: finds a site's feed, gives the token, checks the proof, connects the feed to the channel and disconnects it. Takes WebSub hubs' calls. The only service besides link-preview-service that fetches addresses people type, and it goes only to public addresses on ports 80 and 443 (`anahita-libs/safehttp`) |
| `syndication-worker` | Reads every connected feed and posts what is new; looks at every proof again once a day; pauses and starts feeds again. Answers no requests |
| `activitypub-service` | The channel as an `Organization` on the fediverse, once its admins turn it on, and its posts |
| `atproto-service` | Its posts on Bluesky, as its own Bluesky account, when one of its admins connects one |

A channel's node is `node.actor.actor-service.channel.v1`, its service
`actor-service`. Syndication-service and its worker are optional: left out,
channels have no feeds, and nothing else changes. A site that offers no
channels says so in `instance.json` ([What a site offers](instance-features.md)).

## Making a channel

`POST /channels/` with a name, a description, a link and who may see it, as
a group is made, by whoever `CHANNELS_FROM` allows (administrators unless
the installation says otherwise). Its handle is made from its name. Whoever
makes it is its first admin and its first follower. In the web app it is
Create channel, from + on the Channels page (`/channels/add`). A website's
feed is connected afterwards, in its settings ([Its feed](#its-feed)).

## What it posts

`channel.json` in `graph-grpc-defaults/actor_features/`: nothing for its
followers to add, and for its admins:

| Kind | A new channel | Its admins |
| --- | --- | --- |
| Notes, photos, videos and sounds | On, where the site offers them | — |
| Articles | Off | May turn on, for long pieces |
| Topics | Off | May turn on, to start a discussion its followers reply in |

Its admins turn articles and topics on and off in its settings (Posts), which
saves the kind among its features (`PATCH /channels/:id/features`, its
`node_types` and `composers` for text-service). A profile's saved kinds are
added to its kind's defaults, never taken from them, so notes, photos and
videos stay on. A site trims this in its own `channel.json`, or switches a
kind off for everything in `instance.json`.

Only its admins, and the site's administrators, write on a channel, whatever
its saved features say (`permissions.CanPostTo`): its followers are refused,
and the web app gives them no + button there.

## Who wrote it

A post its admins make on a channel, of any kind, is the channel's own: its
owner and author are the channel, so it is shown and sent as the channel's,
with its name and profile photo. Which admin wrote it is kept in its meta as
`written_by` (`handlers.OwnPost`).

`written_by` is sent, as a short profile, to the channel's admins and to the
site's administrators, on the post and on the replies made as the channel,
in feeds, on its page and in its thread; the web app shows "Written by" and
the admin's name under the time. Nobody else is told, and it is never sent
to another server. A channel's posts are sent without `editor`, which would
name the admin who changed them last. Those who handle reports see it in a
report's case.

## Replies as the channel

Under a channel's own post, a reply in words by one of its admins is the
channel's: its owner and author are the channel, so it answers its readers
as itself, and which admin wrote it is kept as `written_by`, as for its
posts. The channel's replies are changed by whoever replies as it. Anywhere
else, and by anybody else, a reply is its writer's.

## Privacy

As a group's ([Profiles](profiles.md)): anyone, anyone signed in, or its
followers only (`PATCH /channels/:id/access`); follow requests, when its
admins allow them; who sees its followers and the profiles it follows
(`/graph-access`); whom it blocked. A stranger is shown its name, handle and
profile photo, to ask from. A channel can be archived, switched off and
deleted as a group can. Rules to agree to before following are a group's
only: a channel's followers only reply.

## Events

A channel hosts events as a group does: `POST /events/` with the channel's
id in `host_group_id`, by one of its admins, whose admins are carried onto
the event; `GET /channels/:id/events/` lists those it hosts. In the web app,
a channel has an Events tab, where its admins add one.

## Its feed

One feed a channel, connected and disconnected by its admins in its
settings (On your site), whoever `CHANNELS_FROM` allows:

1. `POST /channels/_connect` with the site's address (`example.com`, or its
   feed's). The feed is the address itself, or the first feed the page names
   in its head with `<link rel="alternate">`. It must be on the site's domain
   or under it: `blog.example.com` is example.com's, `feeds.example.net` is
   not. A feed that another channel reads already is refused. The answer is
   the token, and where to put it.
2. `POST /channels/_connect/:id/check` looks for the token, as often as asked.
3. `PUT /channels/_connect/feeds/:channel_id` with the connection and the
   feed connects it: the feed's ten newest items are posted at once, each
   with its own date, so that its followers see the feed has come; the rest
   are marked seen, and from then on only what is published is posted.
   Where the channel names no website, the proved site becomes its website.

A connection is the person's who began it: anybody else is told it is not
there. `GET` at the same address shows the feed, and `DELETE` disconnects
it: the feed is read no more, the notes it made stay, and the items seen are
kept, so that connecting the same feed again posts none of them twice. A
channel that reads a feed already is refused another (`has_feed`).

**A private feed is refused** (`private_feed`): one behind a login, which
answers 401 or 403, and one whose address carries a secret, a user and
password, or a parameter such as `token`, `key`, `secret` or `auth` (a paid
podcast's feed, a private repository's). A feed's notes are public and go to
the fediverse, where they cannot be called back, so what a feed keeps for
some is never made public here.

## Proving a domain

The token, `anahita-` and 32 hexadecimal characters, goes in either place:

| Where | What |
| --- | --- |
| A TXT record | `_anahita.<domain>`, holding the token |
| A file | `https://<host>/.well-known/anahita-verification.txt`, the token on a line of its own |

The domain is the site's host without a leading `www.`; the file is read from
the host as given. The record is looked at first. The proof is
`anahita-libs/domains`, built to be shared: people and groups are to prove
domains the same way ([roadmap row 63](plans/verified-domains.md)).

## Reading the feed

Every 30 minutes (`CHANNEL_FEED_READ_EVERY`), with the `ETag` and `Last-Modified`
the server sent the last time, so an unchanged feed is not sent again. At
most 5 MB, 10 seconds and 3 redirects. A feed that fails is read again 15
minutes later, then 30, an hour, and so on to once a day; how often it has
failed and why are kept in `channel_feeds`. A feed is taken by one worker at
a time, so more than one may run.

## The feed's notes

Each new item becomes a note of the channel (the user, 2026-10-16: a site's
item, shown here, is a short post pointing elsewhere, as a person sharing a
link is, and not writing done here):

| Of the note | From the item |
| --- | --- |
| Its words | The item's title in bold, as a note has no title of its own; its excerpt (the first 300 characters of its text, plain, cut at a word); the original's address; its categories as hashtags, one word each (`Open Science` is `#OpenScience`), five at most. What Markdown would read as marks in the title or the excerpt is kept as written |
| Card | The original's, from link-preview-service: the one card a note carries |
| Date | The item's, unless it says the future |
| Audience | Public |

Its author is the channel; it is made as the channel's first admin, who is
kept in it as `written_by`, and it is marked `from_feed` in its meta (for
the notes made before 2026-11-16, by migration `20261122000022`). At most 10
an hour (`CHANNEL_FEED_HOURLY_CAP`), oldest first; the rest wait for the
next hour. Items are known by their id, or their address when they have
none, and none is posted twice. One whose title or excerpt changes changes
its note; one that leaves the feed leaves its note as it is, since feeds
seldom say that something was deleted.

**A feed's notes follow its site.** Nobody changes their words or narrows
who sees them, its admins and site administrators included: both are
refused with `follows_its_website`, and the web app offers neither. They
are deleted, and replies under them hidden, as any post's; who may reply and
quote stays the admins' to choose. What its admins write on the channel is
theirs to change.

## On the fediverse

A public channel is on the fediverse once one of its admins turns it on in
its settings (Fediverse), and off until then, as a person is: sharing is
chosen, never assumed. One that is not public, or is switched off or
deleted, is not (`profile_not_public`); one that becomes private goes as a
person who does: its posts are taken back elsewhere. Turning it off asks
first, and tells every server that knows it to remove it. While it is on,
WebFinger finds it by its handle (`@garden@example.org`), and it is an
`Organization`, not a `Service`, which Mastodon marks as automated.

Its posts go out as a person's do ([Federation](federation.md)): notes,
photos, videos, sounds and articles, the channel's; topics stay here. A
feed's notes go out as `Note`s with their card, which is how Mastodon shows
a post best. Replies to them come back into its threads, and its own replies
go out. Only what it posts after being turned on goes out. It is `GET` and
`PUT /activitypub/settings?person_id=` with the channel's id, for one of its
admins, as for a person about themselves; anybody else, a site administrator
among them, is refused with `403`.

## On Bluesky

A channel is followed on Bluesky through its own Bluesky account, which one
of its admins connects in its settings (Fediverse), with a password made for
an app, as a person connects theirs ([Federation](federation.md#bluesky)).
Its domain can be its handle there (`@example.com`), proved on Bluesky's side
by a DNS record. From then on its posts are posted there too, as that
account, and people on Bluesky follow it there; its replies under posts from
Bluesky go there as it. What the account posts on Bluesky itself is not made
a post of the channel here: a channel's posts are made here, by its admins
or from its feed. It is `PUT /atproto/settings` with `profile_id`, for one
of its admins; anybody else is refused with `bluesky_not_yours`.

Its handle here has no dot (`@example`), on purpose: here, a dot in a
handle is somebody on Bluesky (`/@ana.bsky.social`), so `@example.com`
would name its Bluesky account, not it.

## WebSub

A feed that names a hub (`rel="hub"`) is subscribed to, its own address
(`rel="self"`) as the topic, for ten days and asked again before they end.
The hub calls back at `/channels/_websub/:id/:signature`, the signature an HMAC
of the channel's id with `SECURE_COOKIE_KEY`: a check is answered with its
challenge, and a push brings the next read forward to now, at most once a
minute. What a hub sends is not read; the feed is always read from the site.
Only where a hub could call back: an installation reachable from the
internet over https, and open to read: a members-only site refuses hubs.
WordPress names a hub only with a plugin. A disconnected feed's hub is
answered that there is no subscription.

## Pausing and starting again

| Paused when | Started again when |
| --- | --- |
| The domain's proof has been gone a week (`proof_gone`) | It is found again, at the next daily look |
| The channel is switched off, deleted (in its grace period) or archived (`profile_gone`) | It is switched on or restored; looked at once an hour |

A paused feed is not read.

## Settings

| Variable | Read by | Default | Means |
| --- | --- | --- | --- |
| `CHANNELS_FROM` | actor-service, syndication-service, auth-service (NodeInfo `channelsFrom`) | `administrators` | The least role that may make a channel and connect a feed: `registered`, `administrators` or `super-administrators`. Give all three the same value. `WEBSITES_FROM`, its old name, is read where it is not set, for one release |
| `CHANNEL_FEED_HOURLY_CAP` | syndication-worker | 10 | The most posts a feed makes in an hour |
| `CHANNEL_FEED_READ_EVERY` | syndication-worker | 30 | How often a feed is read, in minutes; 15 at least |
| `LINK_PREVIEW_URL` | syndication-worker | `http://link-preview-service:80` | Where the original's card is asked for; unset, notes have none |
| `SECURE_COOKIE_KEY`, `DOMAIN_NAME`, `PUBLIC_*` | syndication-service, syndication-worker | | The key the WebSub signature is made with, and the address hubs call back at |

## The tables

The channel is a node, as every profile is. Its feed is in three tables of
syndication-service's (migrations `20261120000001` and `20261121000001`,
renamed by `20261122000021`).

| Table | Holds |
| --- | --- |
| `verified_domains` | A domain being proved or proved: who proves it, for which profile, the token, how and when it was proved, last looked at, failing since |
| `channel_feeds` | One row per connected feed: its address (unique), its site, its domain, its state, what the feed's server said last, the WebSub hub, when it is read next, failures, this hour's posts |
| `channel_feed_items` | Every item seen, by its key, with the note it became and a hash of its title and excerpt: `seen`, `waiting` or `posted`. Kept when the feed is disconnected |

## Endpoints

| Endpoint | Who | What |
| --- | --- | --- |
| `GET /channels/`, `GET /channels/:id` | anybody who may see them | The channels, and one |
| `POST /channels/` | `CHANNELS_FROM` | A channel: `actor.ChannelNode` |
| `PATCH /channels/:id` | its admins | Its name, description and link |
| `PATCH /channels/:id/access`, `/graph-access`, `/features` | its admins | Who sees it and its follows; what it posts |
| `GET /channels/:id/events/` | anybody who may see it | The events it hosts |
| `POST /channels/_connect` | `CHANNELS_FROM` | A site's feed, and the token |
| `POST /channels/_connect/:id/check` | whoever began it | `{ proved, method }` |
| `GET`, `PUT`, `DELETE /channels/_connect/feeds/:channel_id` | its admins; `PUT` also `CHANNELS_FROM` | Its feed: shown, connected, disconnected |
| `GET`, `POST /channels/_websub/:id/:signature` | a hub | A check, and a push |
| `DELETE /channels/:id`, `POST /channels/:id/restore`, `/archive`, `/admins/` | as for a group | |

Refusals of the feed's say why as `error`: `not_a_domain`, `no_feed`,
`unreadable`, `feed_not_on_domain`, `private_feed` (400); `feed_taken`,
`has_feed`, `not_proved` (409). The full reference is
[`docs/api/`](api/index.html#syndication-service).

## What is not built

1. A reply with pictures, a GIF or a video by an admin under a channel's
   post is still their own, since its files are uploaded for them.
2. An admin is not told when their channel's feed is paused, or why.
3. Notes, photos and videos cannot be turned off for one channel; a site
   turns a kind off for all in `channel.json` or `instance.json`.
4. More than one feed a channel, and rules for a channel, until somebody
   asks.

## Where the code is

| What | Where |
| --- | --- |
| The channel as a profile | `actor-service/cmd/api/handlers/channel.go`, `group*.go` |
| What it posts | `graph-grpc-defaults/actor_features/channel.json`; `anahita-libs/permissions/capabilities.go` (`CanPostTo`) |
| Posts and replies as the channel | `anahita-libs/handlers/reply_owner.go` (`OwnPost`, `OwnReply`), `anahita-libs/permissions/reply.go` (`RepliesAs`) |
| Who wrote it | `anahita-libs/handlers/written_by.go`, `anahita-libs/responses/medium.go` and `node.go` (`written_by`) |
| Proving a domain | `anahita-libs/domains` |
| Its feed, WebSub calls | `syndication-service/cmd/api` (`connect.go`, `channel_feed.go`) |
| Finding and reading feeds | `syndication-service/feeds` |
| The worker | `syndication-service/worker`, `syndication-service/cmd/worker` |
| WebSub | `syndication-service/websub` |
| On the fediverse | `activitypub-service` (`isChannel`) |
| A feed's notes follow their site | `anahita-libs/permissions/medium.go` (`IsChannelFeedItem`) |
| The web app | `src/containers/channels`, `src/containers/actors/Settings/ChannelPostKinds.jsx`, `src/components/WrittenBy.jsx`, `src/api/channelConnect.js` (anahita-web) |
